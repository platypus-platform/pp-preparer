package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"github.com/platypus-platform/pp-kv-consul"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"reflect"
	"sync"
	"testing"
)

func init() {
	setLogLevel("FATAL")
}

func tempDir() string {
	dir, err := ioutil.TempDir("", "preparer-test")
	if err != nil {
		panic(err)
	}
	return dir
}

func prepareStore() *ppkv.Client {
	kv, _ := ppkv.NewClient()
	_, err := kv.Get("test")
	if err != nil {
		return nil
	}

	kv.DeleteTree("nodes/testhost")
	kv.DeleteTree("clusters/testapp")

	return kv
}

func TestReadsConfigFromStore(t *testing.T) {
	kv := prepareStore()
	if kv == nil {
		t.Skip("KV store not available, skipping test.")
		return
	}

	kv.Put("nodes/testhost/testapp", map[string]string{
		"cluster": "test",
	})
	kv.Put("clusters/testapp/test/versions", map[string]string{
		"abc123": "prep",
		"def456": "active",
	})
	kv.Put("clusters/testapp/test/deploy_config", map[string]string{
		"basedir": "/sometmp",
	})

	c := make(chan WorkSpec)
	s := make([]WorkSpec, 0)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for w := range c {
			// This is kind of a roundabout way to get the results into a slice.
			// Surely there is a better way?
			s = append(s, w)
		}
	}()
	config := PreparerConfig{
		Hostname: "testhost",
	}
	err := pollOnce(config, c)
	close(c)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	wg.Wait()

	expected := []WorkSpec{
		WorkSpec{App: "testapp", Version: "abc123", Basedir: "/sometmp"},
		WorkSpec{App: "testapp", Version: "def456", Basedir: "/sometmp"},
	}
	if !reflect.DeepEqual(expected, s) {
		t.Errorf("\nwant: %v\n got: %v", expected, s)
	}
}

func TestGracefullyHandlesNoData(t *testing.T) {
	kv := prepareStore()
	if kv == nil {
		t.Skip("KV store not available, skipping test.")
		return
	}

	config := PreparerConfig{Hostname: "testhost"}
	pollOnce(config, nil)
}

func TestGracefullyInvalidNodeData(t *testing.T) {
	var buf bytes.Buffer
	setOut(&buf)
	defer setOut(defaultOut())

	kv := prepareStore()
	if kv == nil {
		t.Skip("KV store not available, skipping test.")
		return
	}

	kv.Put("nodes/testhost/testapp", 34)

	config := PreparerConfig{Hostname: "testhost"}
	pollOnce(config, nil)

	AssertInclude(t, buf.String(), "Invalid node data")
	AssertInclude(t, buf.String(), "testapp")
}

func TestGracefullyMissingClusterData(t *testing.T) {
	var buf bytes.Buffer
	setOut(&buf)
	defer setOut(defaultOut())

	kv := prepareStore()
	if kv == nil {
		t.Skip("KV store not available, skipping test.")
		return
	}

	kv.Put("nodes/testhost/testapp", map[string]string{
		"bogus": "test",
	})

	config := PreparerConfig{Hostname: "testhost"}
	pollOnce(config, nil)

	AssertInclude(t, buf.String(), "No cluster key")
	AssertInclude(t, buf.String(), "testapp")
}

func TestPrepareArtifactExtractsToInstall(t *testing.T) {
	basedir := tempDir()
	defer os.RemoveAll(basedir)

	repo := tempDir()
	defer os.RemoveAll(repo)

	preparerConfig := PreparerConfig{
		ArtifactRepo: url.URL{Scheme: "file", Path: repo},
	}
	artifactSourcePath := path.Join(repo, "testapp", "testapp_abc123.tar.gz")

	writeTarGz(artifactSourcePath, map[string]string{
		"README": "Hello",
	})

	PrepareArtifact("testapp", "abc123", basedir, preparerConfig)
	expectedFile := path.Join(basedir, "installs", "testapp_abc123", "README")

	contents, err := ioutil.ReadFile(expectedFile)

	if err != nil {
		t.Errorf("Did not extract file: %s", err.Error())
		return
	}
	if string(contents) != "Hello" {
		t.Errorf("README did not contain correct content")
		return
	}
}

func writeTarGz(dest string, files map[string]string) {
	os.MkdirAll(path.Dir(dest), 0755)

	f, _ := os.Create(dest)
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))
	}
}

func TestPrepareArtifactDoesNotOverrideExisting(t *testing.T) {
	basedir := tempDir()
	defer os.RemoveAll(basedir)

	keepFile := path.Join(basedir, "installs/testapp_abc123/keep")
	os.MkdirAll(keepFile, 0755)

	PrepareArtifact("testapp", "abc123", basedir, PreparerConfig{})

	if _, err := os.Stat(keepFile); os.IsNotExist(err) {
		t.Errorf("Overwrote existing file")
	}
}
