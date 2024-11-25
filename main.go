package main

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/Luzifer/go_helpers/v2/str"
	"github.com/Luzifer/rconfig/v2"
	"github.com/Luzifer/scs-extract/scs"
	"github.com/sirupsen/logrus"
)

const dirPermissions = 0x750

var (
	cfg = struct {
		Dest           string `flag:"dest,d" default:"." description:"Path prefix to use to extract files to"`
		Extract        bool   `flag:"extract,x" default:"false" description:"Extract files (if not given files are just listed)"`
		LogLevel       string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
)

func initApp() (err error) {
	if err = rconfig.ParseAndValidate(&cfg); err != nil {
		return fmt.Errorf("parsing CLI options: %w", err)
	}

	l, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("parsing log-level: %w", err)
	}
	logrus.SetLevel(l)

	return nil
}

//nolint:gocyclo // simple loop routine, fine to understand
func main() {
	var err error
	if err = initApp(); err != nil {
		logrus.WithError(err).Fatal("initializing app")
	}

	if cfg.VersionAndExit {
		fmt.Printf("scs-extract %s\n", version) //nolint:forbidigo
		os.Exit(0)
	}

	var (
		archive string
		extract []string
	)

	switch len(rconfig.Args()) {
	case 1:
		// No positional arguments
		logrus.Fatal("no SCS archive given")

	case 2: //nolint:mnd
		archive = rconfig.Args()[1]

	default:
		archive = rconfig.Args()[1]
		extract = rconfig.Args()[2:]
	}

	f, err := os.Open(archive) //#nosec:G304 // Intended to open arbitrary files
	if err != nil {
		logrus.WithError(err).Fatal("opening input file")
	}
	defer f.Close() //nolint:errcheck // will be closed by program exit

	r, err := scs.NewReader(f)
	if err != nil {
		logrus.WithError(err).Fatal("reading SCS file headers")
	}

	logrus.WithField("no_files", len(r.Files)).Debug("opened archive")

	destInfo, err := os.Stat(cfg.Dest)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.WithError(err).Fatal("accessing destination")
		}

		if err := os.MkdirAll(cfg.Dest, dirPermissions); err != nil {
			logrus.WithError(err).Fatal("creating destination directory")
		}
	}

	if destInfo != nil && !destInfo.IsDir() {
		logrus.Fatal("destination exists and is no directory")
	}

	for _, file := range r.Files {
		if !str.StringInSlice(file.Name, extract) && len(extract) > 0 {
			// Files to extract are given but this is not mentioned
			continue
		}

		if file.IsDirectory {
			// Don't care about directories, if they contain files they will be created
			continue
		}

		if !cfg.Extract {
			// Not asked to extract, do not extract
			fmt.Println(file.Name) //nolint:forbidigo // Intended to print file list
			continue
		}

		destPath := path.Join(cfg.Dest, file.Name)
		if err := os.MkdirAll(path.Dir(destPath), dirPermissions); err != nil {
			logrus.WithError(err).Fatal("creating directory")
		}

		src, err := file.Open()
		if err != nil {
			logrus.WithError(err).Fatal("opening file from archive")
		}

		dest, err := os.Create(destPath) //#nosec:G304 // Intended to create files at given location
		if err != nil {
			logrus.WithError(err).Fatal("creating destination file")
		}

		if _, err = io.Copy(dest, src); err != nil {
			logrus.WithError(err).WithField("name", file.Name).Fatal("Unable to write file contents")
		}

		dest.Close() //nolint:errcheck,gosec,revive // Will be closed by program exit
		src.Close()  //nolint:errcheck,gosec // Will be closed by program exit

		logrus.WithField("file", file.Name).Info("File extracted")
	}
}
