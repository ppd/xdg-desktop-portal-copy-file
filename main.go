package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/godbus/dbus"
)

func copyFile(src string, dest string) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}
	return nil
}

type xdgDesktopPortal struct {
	bus *dbus.Conn
	obj dbus.BusObject
}

func desktopPortal(conn *dbus.Conn) xdgDesktopPortal {
	obj := conn.Object(
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop")
	return xdgDesktopPortal{conn, obj}
}

func (p *xdgDesktopPortal) saveFile(folder string, name string) error {
	dest := []byte(folder + "\000")
	options := map[string]dbus.Variant{
		"current_name":   dbus.MakeVariant(name),
		"current_folder": dbus.MakeVariant(dest)}
	call := p.obj.Call(
		"org.freedesktop.portal.FileChooser.SaveFile",
		0,
		"",
		"Choose Location",
		options)
	if call.Err != nil {
		return call.Err
	}
	return nil
}

func (p *xdgDesktopPortal) awaitResponse() (uint, map[string]dbus.Variant, error) {
	ch := make(chan *dbus.Signal)
	p.bus.Signal(ch)
	signal := <-ch

	if signal.Name != "org.freedesktop.portal.Request.Response" {
		return 0, map[string]dbus.Variant{}, errors.New("Unexpected response")
	}

	var response uint
	var results map[string]dbus.Variant
	err := dbus.Store(signal.Body, &response, &results)
	if err != nil {
		return 0, map[string]dbus.Variant{}, err
	}
	return response, results, nil
}

func parseFileChooserResults(results map[string]dbus.Variant) (*url.URL, error) {
	_uris, ok := results["uris"]
	if !ok {
		return &url.URL{}, errors.New("No uris in results")
	}
	uris := _uris.Value().([]string)
	if len(uris) != 1 {
		return &url.URL{}, errors.New("len(uris) != 1")
	}
	return url.Parse(uris[0])
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func parseArgs() (string, string, string) {
	if len(os.Args) < 2 {
		fmt.Println("Syntax: xdg-desktop-portal-copy-file source [target-directory] [target-name]")
		os.Exit(1)
	}
	src := os.Args[1]

	if !fileExists(src) {
		panic("Sourcefile does not exist")
	}

	folder := "~"
	if len(os.Args) >= 3 {
		folder = os.Args[2]
	}

	_, srcfile := filepath.Split(src)
	name := srcfile

	if len(os.Args) >= 4 {
		name = os.Args[3]
	}
	return src, folder, name
}

func main() {
	src, folder, name := parseArgs()

	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}

	portal := desktopPortal(conn)

	err = portal.saveFile(folder, name)

	if err != nil {
		panic(err)
	}

	// wait for response
	response, results, err := portal.awaitResponse()

	if err != nil {
		panic(err)
	}

	if response != 0 {
		os.Exit(0)
	}

	// write to file
	url, err := parseFileChooserResults(results)

	if err != nil {
		panic(err)
	}

	err = copyFile(src, url.Path)
	if err != nil {
		panic(err)
	}
}
