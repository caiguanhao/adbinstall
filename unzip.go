package main

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

func unzipFile(ctx context.Context, zipfile string) error {
	r, err := zip.OpenReader(zipfile)
	if err != nil {
		return err
	}
	defer r.Close()
	var total uint64
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		total += f.UncompressedSize64
	}
	var done uint64
	for _, f := range r.File {
		progChan := make(chan int64)
		go func() {
			var add uint64
			for c := range progChan {
				add = uint64(c)
				progressBar.SetValue(int((done + add) * 10000 / total))
				downloadStatus.SetText("Extracting " + f.Name)
			}
			done += add
		}()
		if err := unzip(ctx, f, progChan); err != nil {
			return err
		}
	}
	return nil
}

func unzip(ctx context.Context, f *zip.File, progChan chan int64) error {
	defer close(progChan)
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	file, err := os.OpenFile(filepath.Join(imageDir, f.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer file.Close()
	reportProgress := func() {
		if fi, err := file.Stat(); err == nil {
			progChan <- fi.Size()
		}
	}
	defer reportProgress()
	done := make(chan bool)
	defer close(done)
	go func() {
		t := time.Tick(100 * time.Millisecond)
		for {
			reportProgress()
			select {
			case <-done:
				return
			case <-t:
			}
		}
	}()
	errc := make(chan error)
	go func() {
		_, err := io.Copy(file, rc)
		errc <- err
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errc:
			return err
		}
	}
}
