package edit

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/hpcng/warewulf/internal/pkg/overlay"
	"github.com/hpcng/warewulf/internal/pkg/util"
	"github.com/hpcng/warewulf/internal/pkg/wwlog"
	"github.com/spf13/cobra"
)


const initialTemplate = `# This is a Warewulf Template file.
#
# This file (suffix '.ww') will be automatically rewritten without the suffix
# when the overlay is rendered for the individual nodes. Here are some examples
# of macros and logic which can be used within this file:
#
# Node FQDN = {{.Id}}
# Node Cluster = {{.ClusterName}}
# Network Config = {{.NetDevs.eth0.Ipaddr}}, {{.NetDevs.eth0.Hwaddr}}, etc.
#
# Go to the documentation pages for more information:
# https://warewulf.org/docs/development/contents/overlays.html
#
# Keep the following for better reference:
# ---
# This file is autogenerated by warewulf
# Host:   {{.BuildHost}}
# Time:   {{.BuildTime}}
# Source: {{.BuildSource}}
`


func CobraRunE(cmd *cobra.Command, args []string) error {
	overlayName := args[0]
	fileName := args[1]

	overlaySourceDir := overlay.OverlaySourceDir(overlayName)
	if !util.IsDir(overlaySourceDir) {
		wwlog.Error("Overlay does not exist: %s", overlayName)
		os.Exit(1)
	}

	overlayFile := path.Join(overlaySourceDir, fileName)
	wwlog.Debug("Will edit overlay file: %s", overlayFile)

	overlayFileDir := path.Dir(overlayFile)
	if CreateDirs {
		err := os.MkdirAll(overlayFileDir, 0755)
		if err != nil {
			wwlog.Error("Could not create directory: %s", overlayFileDir)
			os.Exit(1)
		}
	} else {
		if !util.IsDir(overlayFileDir) {
			wwlog.Error("%s does not exist. Use '--parents' option to create automatically.", overlayFileDir)
			os.Exit(1)
		}
	}

	tempFile, tempFileErr := os.CreateTemp("", "ww-overlay-edit-")
	if tempFileErr != nil {
		wwlog.Error("Unable to create temporary file for editing: %s", tempFileErr)
		os.Exit(1)
	}
	defer os.Remove(tempFile.Name())
	wwlog.Debug("Using temporary file %s", tempFile.Name())

	if util.IsFile(overlayFile) {
		originalFile, openErr := os.Open(overlayFile)
		if openErr != nil {
			wwlog.Error("Unable to open %s: %s", overlayFile, openErr)
			os.Exit(1)
		}
		if _, err := io.Copy(tempFile, originalFile); err != nil {
			wwlog.Error("Unable to copy %s to %s for editing: %s", originalFile.Name(), tempFile.Name(), err)
			os.Exit(1)
		}
		originalFile.Close()
	} else if filepath.Ext(overlayFile) == ".ww" {
		if _, err := tempFile.Write([]byte(initialTemplate)); err != nil {
			wwlog.Error("Unable to write to %s: %s", tempFile.Name(), err)
			os.Exit(1)
		}
	}
	tempFile.Close()

	var startTime time.Time
	if fileInfo, err := os.Stat(tempFile.Name()); err != nil {
		wwlog.Error("Unable to stat %s: %s", tempFile.Name(), err)
		os.Exit(1)
	} else {
		startTime = fileInfo.ModTime()
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "/bin/vi"
	}
	if editorErr := util.ExecInteractive(editor, tempFile.Name()); editorErr != nil {
		wwlog.Error("Editor process exited with an error: %s", editorErr)
		os.Exit(1)
	}

	tempFileReader, tempFileErr := os.Open(tempFile.Name())
	if tempFileErr != nil {
		wwlog.Error("Unable to open %s: %s", tempFile.Name(), tempFileErr)
		os.Exit(1)
	}
	defer tempFileReader.Close()

	if fileInfo, err := os.Stat(tempFile.Name()); err != nil {
		wwlog.Error("Unable to stat %s: %s", tempFile.Name(), err)
		os.Exit(1)
	} else {
		if startTime == fileInfo.ModTime() {
			wwlog.Debug("No change detected. Not updating overlay.")
			os.Exit(0)
		}
	}

	destination, destinationErr := os.OpenFile(overlayFile, os.O_RDWR|os.O_CREATE, os.FileMode(PermMode))
	if destinationErr != nil {
		wwlog.Error("Unable to update %s: %s", overlayFile, destinationErr)
		os.Exit(1)
	}
	defer destination.Close()

	wwlog.Debug("Copy %s to %s", tempFileReader.Name(), destination.Name())
	if _, copyErr := io.Copy(destination, tempFileReader); copyErr != nil {
		wwlog.Error("Unable to update %s: %s", destination.Name(), copyErr)
		os.Exit(1)
	}

	return nil
}
