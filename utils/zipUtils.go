package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"fmt"
)

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func CopyFile(dstName, srcName string) (written int64, err error) {
	src, err := os.Open(srcName)
	if err != nil {
		return
	}
	defer src.Close()
	dst, err := os.OpenFile(dstName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer dst.Close()
	return io.Copy(dst, src)
}

//copy dir's all files to dst dir
func CopyFiles(dstDir, srcDir string) (error) {
	err := filepath.Walk(srcDir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			_, err := CopyFile(dstDir + "/" + f.Name(), srcDir + "/" + f.Name())
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

//copy the https://golangcode.com/create-zip-files-in-go
func ZipFiles(filename string, files []string) error {
	fmt.Println(filename)
	fmt.Println(files)
	newfile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer newfile.Close()

	zipWriter := zip.NewWriter(newfile)
	defer zipWriter.Close()

	// Add files to zip
	for _, file := range files {

		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()

		// Get the file information
		info, err := zipfile.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, zipfile)
		if err != nil {
			return err
		}
	}
	return nil
}