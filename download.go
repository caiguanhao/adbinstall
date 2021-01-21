package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var (
	progressBar    *walk.ProgressBar
	urlComboBox    *walk.ComboBox
	downloadButton *walk.PushButton
	downloadStatus *walk.TextLabel
	cancelDownload func()
)

type progress struct {
	downloaded int64
	total      int64
}

func showDownloader() {
	os.MkdirAll(imageDir, 0755)
	var downloader *walk.Dialog
	absPath, _ := filepath.Abs(imageDir)
	truncated := truncatePath(absPath, 30)
	Dialog{
		AssignTo:  &downloader,
		Layout:    VBox{},
		Title:     "Downloader",
		MinSize:   Size{500, 120},
		FixedSize: true,
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					TextLabel{
						Text:          "Location:",
						StretchFactor: 1,
						TextAlignment: AlignHNearVCenter,
					},
					Composite{
						StretchFactor: 5,
						Layout: VBox{
							MarginsZero: true,
							SpacingZero: true,
						},
						Children: []Widget{
							LinkLabel{
								MaxSize: Size{
									Height: 12,
								},
								Alignment:   AlignHNearVCenter,
								ToolTipText: absPath,
								Text:        fmt.Sprintf(`<a href="%s">%s</a>`, absPath, truncated),
								OnLinkActivated: func(link *walk.LinkLabelLink) {
									exec.Command("explorer.exe", link.URL()).Run()
								},
							},
							TextLabel{
								StretchFactor: 1,
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						Text:          "URL:",
						StretchFactor: 1,
						TextAlignment: AlignHNearVCenter,
					},
					Composite{
						StretchFactor: 5,
						Layout: VBox{
							MarginsZero: true,
						},
						Children: []Widget{
							ComboBox{
								AssignTo:     &urlComboBox,
								CurrentIndex: 0,
								MaxSize: Size{
									Width: 1,
								},
								Model: []string{
									"https://zima.oss-cn-hongkong.aliyuncs.com/images/zima/SW_SD5300_V046_A03_fastboot.zip",
								},
								Editable: true,
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						StretchFactor: 1,
					},
					HSplitter{
						StretchFactor: 5,
						Children: []Widget{
							PushButton{
								AssignTo: &downloadButton,
								Text:     "DOWNLOAD",
								OnClicked: func() {
									download()
								},
							},
							TextLabel{
								AssignTo:      &downloadStatus,
								TextAlignment: AlignHNearVCenter,
								Text:          "Ready",
								StretchFactor: 3,
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						Text:          "Progress:",
						StretchFactor: 1,
						TextAlignment: AlignHNearVCenter,
					},
					Composite{
						StretchFactor: 5,
						Layout: VBox{
							MarginsZero: true,
						},
						Children: []Widget{
							ProgressBar{
								AssignTo: &progressBar,
								MaxValue: 10000,
								MinValue: 0,
							},
						},
					},
				},
			},
		},
	}.Create(md)
	updateDialog(downloader)
	downloader.Run()
	if cancelDownload != nil {
		cancelDownload()
	}
}

func download() {
	if cancelDownload != nil {
		cancelDownload()
		cancelDownload = nil
		downloadButton.SetText("DOWNLOAD")
		return
	}
	url := urlComboBox.Text()
	file := filepath.Join(imageDir, "tmp")
	progChan := make(chan progress)
	go func() {
		for prog := range progChan {
			progressBar.SetValue(int(prog.downloaded * 10000 / prog.total))
			downloadStatus.SetText(fmt.Sprintf("Received %s out of %s", formatSize(prog.downloaded), formatSize(prog.total)))
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancelDownload = cancel
	downloadButton.SetText("STOP")
	go func() {
		err := downloadFile(ctx, url, file, progChan)
		if err == nil {
			err = unzipFile(ctx, file)
		}
		if err != nil && err != context.Canceled {
			os.Remove(file)
			walk.MsgBox(md, "Error", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
		}
		if err == nil {
			os.Remove(file)
			downloadStatus.SetText("Done")
		}
		cancelDownload = nil
		downloadButton.SetText("DOWNLOAD")
	}()
}

func downloadFile(ctx context.Context, url, file string, progChan chan progress) error {
	if progChan != nil {
		defer close(progChan)
	}
	err := os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	client := &http.Client{}
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(hctx, "HEAD", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	reportProgress := func() {
		if progChan == nil {
			return
		}
		if fi, err := f.Stat(); err == nil {
			progChan <- progress{
				downloaded: fi.Size(),
				total:      total,
			}
		}
	}
	defer reportProgress()
	var start int64 = 0
	fileSize := fi.Size()
	if fileSize > 0 && strings.Contains(resp.Header.Get("Accept-Ranges"), "bytes") {
		if fileSize < total {
			start = fileSize
		} else if fileSize == total {
			return nil
		}
	}
	if start == 0 {
		err := os.Truncate(file, 0)
		if err != nil {
			return err
		}
	}
	req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if start > 0 {
		req.Header.Add("Range", fmt.Sprintf("bytes=%d-", start))
	}
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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
	_, err = io.Copy(f, resp.Body)
	return err
}

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func truncatePath(path string, size int) (truncated string) {
	if len(path) <= size {
		truncated = path
		return
	}
	parts := strings.Split(path, string(filepath.Separator))
	for i := 0; len(truncated) < size; i++ {
		a, b := i+1, len(parts)-1-i
		if a >= b {
			truncated = path
			break
		}
		t := append(append(append([]string{}, parts[:a]...), "..."), parts[b:]...)
		truncated = strings.Join(t, string(filepath.Separator))
	}
	return
}
