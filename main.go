package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"github.com/armon/consul-api" // TODO: Lock to branch
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
)

type NodeApp struct {
	Cluster string
}

type DeployConfig struct {
	Basedir string
	Runas   string
}

type PreparerConfig struct {
	ArtifactRepo url.URL
}

func main() {
	client, err := consulapi.NewClient(consulapi.DefaultConfig())
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}
	hostname, err := os.Hostname()
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}

	kv := client.KV()
	apps, _, err := kv.List("nodes/"+hostname, nil)
	if err != nil {
		panic(err)
	}
	for _, app := range apps {
		var nodeApp NodeApp
		err := json.Unmarshal(app.Value, &nodeApp)
		if err != nil {
			Fatal("Invalid nodes data for %s: %s", app.Key, err)
			continue
		}
		versionData, _, err :=
			kv.Get("clusters/"+nodeApp.Cluster+"/versions", nil)

		var versionSpec map[string]string
		err = json.Unmarshal(versionData.Value, &versionSpec)
		if err != nil {
			Fatal("Invalid version data for %s: %s", nodeApp.Cluster, err)
			continue
		}

		var deployConfig DeployConfig
		configData, _, err :=
			kv.Get("clusters/"+nodeApp.Cluster+"/deploy_config", nil)
		err = json.Unmarshal(configData.Value, &deployConfig)
		if err != nil {
			Fatal("Invalid deploy config data for %s: %s", nodeApp.Cluster, err)
			continue
		}
		preparerConfig := PreparerConfig{
			ArtifactRepo: url.URL{Scheme: "file", Path: "/tmp/local-hoist-repo"},
		}

		for version, _ := range versionSpec {
			PrepareArtifact(path.Base(app.Key), version, deployConfig, preparerConfig)
		}
	}
}

func PrepareArtifact(app string, version string, deployConfig DeployConfig, preparerConfig PreparerConfig) {
	targetDir := path.Join(deployConfig.Basedir, "installs", app+"_"+version)
	tmpDir, err := ioutil.TempDir("", "sp-preparer")

	if err != nil {
		Fatal("Could not create temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		Info("Does not exist: %s", targetDir)
		artifactPath := path.Join(preparerConfig.ArtifactRepo.Path, app, app+"_"+version+".tar.gz")
		Info("Fetching artifact")
		Info("Extracting %s to %s", artifactPath, tmpDir)
		err := extractTarGz(artifactPath, tmpDir)
		if err != nil {
			Fatal("Could not extract %s to %s: %s", artifactPath, targetDir, err.Error())
			return
		}
		Info("Moving %s to %s", tmpDir, targetDir)
		os.MkdirAll(path.Dir(targetDir), 0755)
		err = os.Rename(tmpDir, targetDir)
		if err != nil {
			Fatal("Could not move %s to %s: %s", tmpDir, targetDir, err.Error())
			return
		}
	} else {
		Info("Already exists: %s", targetDir)
	}
}

func extractTarGz(src string, dest string) (err error) {
	fi, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fi.Close()

	fz, err := gzip.NewReader(fi)
	if err != nil {
		return err
	}
	defer fz.Close()

	tr := tar.NewReader(fz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}
		fpath := path.Join(dest, hdr.Name)
		if hdr.FileInfo().IsDir() {
			continue
		} else {
			dir := path.Dir(fpath)
			os.MkdirAll(dir, 0755)
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, tr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
