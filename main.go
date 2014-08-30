package main

import (
	"archive/tar"
	"compress/gzip"
	. "github.com/platypus-platform/pp-logging"
	"github.com/platypus-platform/pp-store"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
)

type PreparerConfig struct {
	ArtifactRepo url.URL
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}

	preparerConfig := PreparerConfig{
		ArtifactRepo: url.URL{Scheme: "file", Path: "/tmp/local-repo"},
	}

	err = pp.PollIntent(hostname, func(intent pp.IntentNode) {
		for _, app := range intent.Apps {
			for version, _ := range app.Versions {
				PrepareArtifact(app.Name, version, app.Basedir, preparerConfig)
			}
		}
	})

	if err != nil {
		Fatal(err.Error())
		os.Exit(1)
	}
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
