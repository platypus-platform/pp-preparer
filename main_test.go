package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"github.com/armon/consul-api" // TODO: Lock to branch
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"reflect"
	"sync"
	"testing"
)

func init() {
	setLogLevel("NONE")
}

func tempDir() string {
	dir, err := ioutil.TempDir("", "preparer-test")
	if err != nil {
		panic(err)
	}
	return dir
}

func Set(kv *consulapi.KV, key string, value map[string]string) {
	body, _ := json.Marshal(value)

	node := &consulapi.KVPair{
		Key:   key,
		Value: body,
	}
	kv.Put(node, nil)
	return
}

func TestReadsConfigFromConsul(t *testing.T) {
	client, _ := consulapi.NewClient(consulapi.DefaultConfig())
	_, _, err := client.KV().Get("test", nil)
	if err != nil {
		t.Skip("Consul not available, skipping test: %s", err.Error())
		return
	}

	kv := client.KV()
	kv.DeleteTree("nodes/testhost", nil)
	kv.DeleteTree("clusters/testapp", nil)
	Set(kv, "nodes/testhost/testapp", map[string]string{
		"cluster": "test",
	})
	Set(kv, "clusters/testapp/test/versions", map[string]string{
		"abc123": "prep",
		"def456": "active",
	})
	Set(kv, "clusters/testapp/test/deploy_config", map[string]string{
		"basedir": "/sometmp",
	})

	c := make(chan WorkSpec)
	s := make([]WorkSpec, 0)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for w := range c {
			// This is kind of a roundabout way to get the results into an array.
			// Surely there is a better way?
			s = append(s, w)
		}
	}()
	config := PreparerConfig{
		Hostname: "testhost",
	}
	err = pollConsulOnce(config, c)
	close(c)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	wg.Wait()

	expected := []WorkSpec{
		WorkSpec{App: "testapp", Version: "abc123", Config: DeployConfig{Basedir: "/sometmp"}},
		WorkSpec{App: "testapp", Version: "def456", Config: DeployConfig{Basedir: "/sometmp"}},
	}
	if !reflect.DeepEqual(expected, s) {
		t.Errorf("\nwant: %v\n got: %v", expected, s)
	}
}

func TestPrepareArtifactExtractsToInstall(t *testing.T) {
	d := tempDir()
	defer os.RemoveAll(d)

	config := DeployConfig{Basedir: d}

	repo := tempDir()
	defer os.RemoveAll(repo)

	preparerConfig := PreparerConfig{
		ArtifactRepo: url.URL{Scheme: "file", Path: repo},
	}
	artifactSourcePath := path.Join(repo, "testapp", "testapp_abc123.tar.gz")

	writeTarGz(artifactSourcePath, map[string]string{
		"README": "Hello",
	})

	PrepareArtifact("testapp", "abc123", config, preparerConfig)
	expectedFile := path.Join(d, "installs", "testapp_abc123", "README")

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
	d := tempDir()
	defer os.RemoveAll(d)

	config := DeployConfig{Basedir: d}
	keepFile := path.Join(d, "installs/testapp_abc123/keep")
	os.MkdirAll(keepFile, 0755)

	PrepareArtifact("testapp", "abc123", config, PreparerConfig{})

	if _, err := os.Stat(keepFile); os.IsNotExist(err) {
		t.Errorf("Overwrote existing file")
	}
}
