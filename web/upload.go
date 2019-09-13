package web

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	MaxUploadSize int64  = 2 << 32;
	UploadPath    string = "./upload"
)

func RenderError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	if _, e := w.Write([]byte(message)); e != nil {
		log.Println(e)
	}
}

func welcome(w http.ResponseWriter, r *http.Request)  {
	hash := md5.New()
	_, err := io.WriteString(hash, strconv.FormatInt(time.Now().Unix(), 10))
	FailOnError(err, "write error")
	data := fmt.Sprintf("%x", hash.Sum(nil))

	userAgent := r.Header.Get("User-Agent")
	t := "web/pages/new_upload.html"
	if strings.Contains(userAgent, "MSIE") {
		t = "web/pages/upload.html"
	}

	if tmp, e := template.ParseFiles(t); e == nil {
		e := tmp.Execute(w, data)
		FailOnError(e, "template render error")
	} else {
		RenderError(w, e.Error(), http.StatusInternalServerError)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request)  {
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		log.Println(err)
		RenderError(w, "FILE_TOO_BIG", http.StatusBadRequest)
		return
	}

	if r.MultipartForm == nil {
		err := r.ParseMultipartForm(32 << 20)
		RenderError(w, err.Error(), http.StatusBadRequest)
	}

	makeUploadDir()

	fileHeaders := r.MultipartForm.File["uploadfile"]
	for _, fileHeader := range fileHeaders {
		saveUploadFile(w, fileHeader)
	}

	//sourceFile, fileHeader, err := r.FormFile("uploadfile")


	welcome(w, r)

}

func saveUploadFile(w http.ResponseWriter, fileHeader *multipart.FileHeader) {
	sourceFile, err := fileHeader.Open()
	if err != nil {
		log.Println(err)
		RenderError(w, "INVALID_FILE", http.StatusBadRequest)
		return
	}
	defer sourceFile.Close()

	//fileBytes, err := ioutil.ReadAll(file)
	//
	//// check file type, detectcontenttype only needs the first 512 bytes
	//filetype := http.DetectContentType(fileBytes)
	//
	//fileEndings, err := mime.ExtensionsByType(fileType)

	targetFile, err := os.OpenFile(filepath.Join(UploadPath, filepath.Base(fileHeader.Filename)), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Println(err)
		RenderError(w, "CANT_WRITE_FILE", http.StatusBadRequest)
		return
	}
	defer targetFile.Close()
	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		log.Println(err)
		RenderError(w, "ERROR_WRITE_FILE", http.StatusBadRequest)
	}
}

func makeUploadDir() {
	if _, err := os.Stat(UploadPath); os.IsNotExist(err) {
		os.Mkdir(UploadPath, os.ModePerm)
	}
}

func UploadFileHanler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		log.Println(method)
		switch method {
		case "GET":
			welcome(w, r)
		case "POST":
			uploadHandler(w, r)
		}
	});
}