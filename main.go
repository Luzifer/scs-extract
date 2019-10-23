package main

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/Luzifer/go_helpers/v2/str"
	"github.com/Luzifer/rconfig/v2"
	"github.com/Luzifer/scs-extract/scs"
	log "github.com/sirupsen/logrus"
)

var (
	cfg = struct {
		Dest           string `flag:"dest,d" default:"." description:"Path prefix to use to extract files to"`
		Extract        bool   `flag:"extract,x" default:"false" description:"Extract files (if not given files are just listed)"`
		LogLevel       string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
)

func init() {
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("scs-extract %s\n", version)
		os.Exit(0)
	}

	if l, err := log.ParseLevel(cfg.LogLevel); err != nil {
		log.WithError(err).Fatal("Unable to parse log level")
	} else {
		log.SetLevel(l)
	}
}

func main() {
	var (
		archive string
		extract []string
	)

	switch len(rconfig.Args()) {
	case 1:
		// No positional arguments
		log.Fatal("No SCS archive given")

	case 2:
		archive = rconfig.Args()[1]

	default:
		archive = rconfig.Args()[1]
		extract = rconfig.Args()[2:]
	}

	f, err := os.Open(archive)
	if err != nil {
		log.WithError(err).Fatal("Unable to open input file")
	}
	defer f.Close()

	r, err := scs.NewReader(f, 0)
	if err != nil {
		log.WithError(err).Fatal("Unable to read SCS file headers")
	}

	log.WithField("no_files", len(r.Files)).Debug("Opened archive")

	destInfo, err := os.Stat(cfg.Dest)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Fatal("Unable to access destination")
		}

		if err := os.MkdirAll(cfg.Dest, 0755); err != nil {
			log.WithError(err).Fatal("Unable to create destination directory")
		}
	}

	if destInfo != nil && !destInfo.IsDir() {
		log.Fatal("Destination exists and is no directory")
	}

	for _, file := range r.Files {
		if !str.StringInSlice(file.Name, extract) && len(extract) > 0 {
			// Files to extract are given but this is not mentioned
			continue
		}

		if file.Type == scs.EntryTypeCompressedNames || file.Type == scs.EntryTypeCompressedNamesCopy ||
			file.Type == scs.EntryTypeUncompressedNames || file.Type == scs.EntryTypeUncompressedNamesCopy {
			// Don't care about directories, if they contain files they will be created
			continue
		}

		if !cfg.Extract {
			// Not asked to extract, do not extract
			fmt.Println(file.Name)
			continue
		}

		destPath := path.Join(cfg.Dest, file.Name)
		if err := os.MkdirAll(path.Dir(destPath), 0755); err != nil {
			log.WithError(err).Fatal("Unable to create directory")
		}

		src, err := file.Open()
		if err != nil {
			log.WithError(err).Fatal("Unable to open file from archive")
		}

		dest, err := os.Create(destPath)
		if err != nil {
			log.WithError(err).Fatal("Unable to create destination file")
		}

		if _, err = io.Copy(dest, src); err != nil {
			log.WithError(err).Fatal("Unable to write file contents")
		}

		dest.Close()
		src.Close()

		log.WithField("file", file.Name).Info("File extracted")
	}
}
