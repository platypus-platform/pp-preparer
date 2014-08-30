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
	Hostname     string
	ArtifactRepo url.URL
}

type WorkSpec struct {
	App     string
	Version string
	Config  DeployConfig
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}

	preparerConfig := PreparerConfig{
		ArtifactRepo: url.URL{Scheme: "file", Path: "/tmp/local-repo"},
		Hostname:     hostname,
	}

	c := make(chan WorkSpec)
	go func() {
		for w := range c {
			PrepareArtifact(w.App, w.Version, w.Config, preparerConfig)
		}
	}()

	err = pollConsulOnce(preparerConfig, c)
	close(c)
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}

}

func pollConsulOnce(config PreparerConfig, c chan WorkSpec) error {
	client, err := consulapi.NewClient(consulapi.DefaultConfig())
	if err != nil {
		return err
	}
	kv := client.KV()
	apps, _, err := kv.List("nodes/"+config.Hostname, nil)
	if err != nil {
		return err
	}
	for _, app := range apps {
		appName := path.Base(app.Key)
		var nodeApp NodeApp
		err := json.Unmarshal(app.Value, &nodeApp)
		if err != nil {
			Fatal("Invalid nodes data for %s: %s", appName, err)
			continue
		}
		clusterKey := path.Join("clusters", appName, nodeApp.Cluster, "versions")
		versionData, _, err := kv.Get(clusterKey, nil)

		if versionData == nil {
			Fatal("No data for %s", clusterKey)
			continue
		}
		var versionSpec map[string]string
		err = json.Unmarshal(versionData.Value, &versionSpec)
		if err != nil {
			Fatal("Invalid version data for %s: %s", nodeApp.Cluster, err)
			continue
		}

		var deployConfig DeployConfig
		configKey := path.Join("clusters", appName, nodeApp.Cluster, "deploy_config")
		configData, _, err := kv.Get(configKey, nil)
		if configData == nil {
			Fatal("No data for %s", configKey)
			continue
		}
		err = json.Unmarshal(configData.Value, &deployConfig)
		if err != nil {
			Fatal("Invalid deploy config data for %s: %s", nodeApp.Cluster, err)
			continue
		}

		for version, _ := range versionSpec {
			c <- WorkSpec{
				App:     appName,
				Version: version,
				Config:  deployConfig,
			}
		}
	}
	return nil
}

func PrepareArtifact(app string, version string, deployConfig DeployConfig, preparerConfig PreparerConfig) {
	targetDir := path.Join(deployConfig.Basedir, "installs", app+"_"+version)
	tmpDir, err := ioutil.TempDir("", "preparer")

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
