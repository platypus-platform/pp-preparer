package main

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"testing"
)

func init() {
	setLogLevel("NONE")
}

func tempDir() string {
	dir, err := ioutil.TempDir("", "sp-preparer-test")
	if err != nil {
		panic(err)
	}
	return dir
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
