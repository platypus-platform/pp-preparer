package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"github.com/platypus-platform/pp-kv-consul"
	. "github.com/platypus-platform/pp-logging"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
)

type PreparerConfig struct {
	Hostname     string
	ArtifactRepo url.URL
}

type WorkSpec struct {
	App     string
	Version string
	Basedir string
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
			PrepareArtifact(w.App, w.Version, w.Basedir, preparerConfig)
		}
	}()

	err = pollOnce(preparerConfig, c)
	close(c)
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}

}

func pollOnce(config PreparerConfig, c chan WorkSpec) error {
	kv, _ := ppkv.NewClient()
	apps, err := kv.List(path.Join("nodes", config.Hostname))
	if err != nil {
		return err
	}

	for appName, data := range apps {
		appData, worked := stringMap(data)
		if !worked {
			Fatal("Invalid node data for %s", appName)
			continue
		}

		cluster := appData["cluster"]
		if cluster == "" {
			Fatal("No cluster key in node data for %s", appName)
			continue
		}

		clusterKey := path.Join("clusters", appName, cluster, "versions")
		configKey := path.Join("clusters", appName, cluster, "deploy_config")

		versions, err := getMap(kv, clusterKey)
		if err != nil {
			Fatal("No or invalid data for %s: %s", clusterKey, err)
			continue
		}

		deployConfig, err := getMap(kv, configKey)
		if err != nil {
			Fatal("No or invalid data for %s: %s", configKey, err)
			continue
		}

		basedir := deployConfig["basedir"]
		if !path.IsAbs(basedir) {
			Fatal("Not allowing relative basedir in %s", configKey)
			continue
		}

		for version, _ := range versions {
			c <- WorkSpec{
				App:     appName,
				Version: version,
				Basedir: basedir,
			}
		}
	}
	return nil
}

func PrepareArtifact(
	app string,
	version string,
	basedir string,
	preparerConfig PreparerConfig,
) {

	targetDir := path.Join(basedir, "installs", app+"_"+version)
	tmpDir, err := ioutil.TempDir("", "preparer")

	if err != nil {
		Fatal("Could not create temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		Info("Does not exist: %s", targetDir)
		artifactPath := path.Join(
			preparerConfig.ArtifactRepo.Path,
			app,
			app+"_"+version+".tar.gz",
		)

		Warn("TODO: Fetching artifact")
		Info("Extracting %s to %s", artifactPath, tmpDir)

		err := extractTarGz(artifactPath, tmpDir)
		if err != nil {
			Fatal("Could not extract %s to %s: %s",
				artifactPath, targetDir, err.Error())
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
		Info("%s already exists, skipping", targetDir)
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

func getMap(kv *ppkv.Client, query string) (map[string]string, error) {
	raw, err := kv.Get(query)

	if err != nil {
		return nil, err
	}

	mapped, worked := stringMap(raw)
	if !worked {
		return nil, errors.New("Not a string map")
	}

	return mapped, nil
}

func stringMap(raw interface{}) (map[string]string, bool) {
	mapped, worked := raw.(map[string]interface{})
	if !worked {
		return nil, false
	}
	ret := map[string]string{}
	for k, v := range mapped {
		str, worked := v.(string)
		if !worked {
			return nil, false
		}
		ret[k] = str
	}
	return ret, true
}
