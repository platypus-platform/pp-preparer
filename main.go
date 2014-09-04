package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"flag"
	"fmt"
	. "github.com/platypus-platform/pp-logging"
	"github.com/platypus-platform/pp-store"
	"gopkg.in/yaml.v1"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
)

type PreparerConfig struct {
	ArtifactRepo ArtifactUrl
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}

	var preparerConfig PreparerConfig
	preparerConfig.ArtifactRepo = ArtifactUrl{
		Scheme: "file",
		Path:   "fake/repo",
	}

	flag.Var(&preparerConfig.ArtifactRepo, "repo", "repo url")
	flag.Parse()

	err = pp.PollIntent(hostname, func(intent pp.IntentNode) {
		for _, app := range intent.Apps {
			for version, _ := range app.Versions {
				PrepareArtifact(app.Name, version, app.Basedir, preparerConfig)
				PrepareConfig(app)
			}
		}
	})

	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}
}

// Need to use a custom type so we can implement flag.Value
type ArtifactUrl url.URL

func (i *ArtifactUrl) String() string {
	var x url.URL
	x = url.URL(*i)
	return x.String()
}

func (i *ArtifactUrl) Set(value string) error {
	parsed, err := url.Parse(value)

	if err != nil {
		return err
	}

	*i = ArtifactUrl(*parsed)
	return nil
}

// TODO: How to clean up old config?
// TODO: Tests
func PrepareConfig(app pp.IntentApp) {
	content, err := yaml.Marshal(&app.UserConfig)
	if err != nil {
		Fatal("%s: could not emit user config as yaml: %s", app.Name, err)
		return
	}
	// YAML output is deterministic, even when using maps.
	sha := fmt.Sprintf("%x", md5.Sum(content))
	targetPath := path.Join(app.Basedir, "configs", sha+".yaml")
	if err := os.MkdirAll(path.Dir(targetPath), 0755); err != nil {
		Fatal("%s: could not write config: %s", app.Name, err)
		return
	}

	Info("%s: writing config", app.Name)
	if err := writeFileAtomic(targetPath, content, 0644); err != nil {
		Fatal("%s: could not write config: %s", app.Name, err)
		return
	}
}

func PrepareArtifact(
	app string,
	version string,
	basedir string,
	preparerConfig PreparerConfig,
) {

	targetDir := path.Join(basedir, "installs", app+"_"+version)
	// TODO: Need to ensure tmpDir is on same filesystem as target, so the move
	// can be atomic. Maybe use basedir/tmp ?
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

// TODO: Duplicated in pp-iptables
func writeFileAtomic(fpath string, fcontent []byte, mod os.FileMode) error {
	f, err := ioutil.TempFile("", "writeFileAtomic")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // This will fail in happy case, that's fine.

	if _, err := f.Write(fcontent); err != nil {
		return err
	}
	if err := f.Chmod(mod); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), fpath); err != nil {
		return err
	}

	return nil
}
