package web

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unsafe"

	"golang.org/x/net/html/charset"
)

type PomPoint struct {
	Project    xml.Name    `xml:"project"`
	GroupId    string      `xml:"groupId"`
	ArtifactId string      `xml:"artifactId"`
	Version    string      `xml:"version"`
	Parent     ParentPoint `xml:"parent"`
}

type Point struct {
	GroupId    string `default: ""`
	ArtifactId string `default: ""`
	Version    string `default: ""`
	Extension  string `"default: "jar"`
}

type ParentPoint struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type SearchResult struct {
	Items []SearchItem `json:"items"`
}

type SearchItem struct {
	Id string `json:"id"`
}

type MavenConfig struct {
	BaseContext, LocalRepoDir, MavenServerHost, RepoName, Username, Secret, ReqHost string
	Msgch                                                                           *chan string
}

func verify(fileName string) bool {
	var sha1Msg string
	//fileName := "D:/.m2/repository/org/springframework/batch/spring-batch-core/3.0.10.RELEASE/spring-batch-core-3.0.10.RELEASE.jar"
	fh, e := os.Open(fileName)
	if e != nil {
		log.Println(e)
		return false
	}
	defer fh.Close()
	jarSha1FileName := fileName + ".sha1"
	if _, e = os.Stat(jarSha1FileName); e == nil {
		fh1, e := os.Open(jarSha1FileName)
		if e != nil {
			log.Println(e)
			return false
		}
		defer fh1.Close()
		buf, e := ioutil.ReadAll(fh1)
		if e != nil {
			log.Println(e)
			return false
		}
		sha1Msg = *(*string)(unsafe.Pointer(&buf))
		sha1Msg = strings.TrimSpace(sha1Msg)
		sha1Msg = sha1Msg[:40]
	} else {
		fmt.Println(fileName, "不存在sha1文件")
		return true
	}
	hash := sha1.New()
	_, e = io.Copy(hash, fh)
	if e != nil {
		log.Println(e)
		return false
	}
	sum := hash.Sum(nil)
	s := hex.EncodeToString(sum)
	return s == sha1Msg
}

func (cfg *MavenConfig) execJarUpload() {
	count := 0
	filepath.Walk(cfg.LocalRepoDir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, e)
			return e
		}
		// filename
		lowName := strings.ToLower(info.Name())
		err, done := cfg.jarFileHandler(&info, lowName, path, &count)
		if done {
			return err
		}
		err, done = cfg.parentFileHandler(&info, lowName, path, &count)
		if done {
			return err
		}

		return nil
	})

	*cfg.Msgch <- fmt.Sprint("count: ", count)
}

func includeJar(path string) bool {
	dir := filepath.Dir(path)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Println(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".jar") {
			return true
		}
	}
	return false
}

// 对于单独的父POM文件进行处理
func (cfg *MavenConfig) parentFileHandler(info *os.FileInfo, lowName string, path string, count *int) (error, bool) {
	if !(*info).IsDir() && !includeJar(path) && strings.HasSuffix(lowName, ".pom") {
		if !verify(path) {
			fmt.Println(path, "pom文件sha1校验失败")
			return nil, true
		}

		point, err, nodone := cfg.getCoordinate(path)
		if nodone {
			return err, nodone
		}

		if len(point.GroupId) != 0 && len(point.ArtifactId) != 0 && len(point.Version) != 0 {
			*cfg.Msgch <- fmt.Sprintf("{%s:%s:%s}", point.GroupId, point.ArtifactId, point.Version)
			cfg.singleRequest(point, path)
		}
		*count++
	}
	return nil, false
}

func (cfg *MavenConfig) jarFileHandler(info *os.FileInfo, lowName string, path string, count *int) (error, bool) {
	if !(*info).IsDir() && strings.HasSuffix(lowName, ".jar") &&
		!strings.HasSuffix(lowName, "-sources.jar") &&
		!strings.HasSuffix(lowName, "-snapshot.jar") &&
		!strings.HasSuffix(lowName, "-javadoc.jar") {

		if !verify(path) {
			fmt.Println(path, "jar文件sha1校验失败")
			return nil, true
		}

		pomFilename := strings.Replace(path, ".jar", ".pom", 1)
		if _, e := os.Stat(pomFilename); os.IsNotExist(e) {
			return nil, true
		}

		point, err, nodone := cfg.getCoordinate(pomFilename)
		if nodone {
			return err, nodone
		}

		if len(point.GroupId) != 0 && len(point.ArtifactId) != 0 && len(point.Version) != 0 {
			*cfg.Msgch <- fmt.Sprintf("{%s:%s:%s}", point.GroupId, point.ArtifactId, point.Version)
			cfg.request(point, path)
		}
		*count++
	}
	return nil, false
}

func (cfg *MavenConfig) getCoordinate(pomFilename string) (*Point, error, bool) {
	buf, e := ioutil.ReadFile(pomFilename)
	failOnError(e, "read file error "+pomFilename)

	point := &PomPoint{}
	decoder := xml.NewDecoder(bytes.NewBuffer(buf))
	decoder.CharsetReader = charset.NewReaderLabel
	e = decoder.Decode(point)

	//e = xml.Unmarshal(buf, point)
	failOnError(e, "unmarshal file error "+pomFilename)

	resolvedPoint := Point{}
	if len(point.ArtifactId) == 0 {
		//fmt.Println("pom.xml artifactId is empty " + path)
		return &resolvedPoint, errors.New("pom.xml artifactId is empty" + pomFilename), true
	} else {
		resolvedPoint.GroupId = point.GroupId
		resolvedPoint.ArtifactId = point.ArtifactId
		resolvedPoint.Version = point.Version
		//g, a, v = point.GroupId, point.ArtifactId, point.Version
	}
	if len(point.GroupId) == 0 {
		resolvedPoint.GroupId = point.Parent.GroupId
	}
	if len(point.Version) == 0 {
		resolvedPoint.Version = point.Parent.Version
	}
	return &resolvedPoint, nil, false
}

const (
	urlStrTmp       string = `http://{{.addr}}{{.base}}/v1/components?repository={{.repo}}`
	searchUrlStrTmp string = `http://{{.addr}}{{.base}}/v1/search?repository={{.repo}}&maven.groupId={{.g}}&maven.artifactId={{.a}}&maven.baseVersion={{.v}}&maven.extension={{.ext}}`
	deleteUrlStrTmp string = `http://{{.addr}}{{.base}}/v1/components/{{.id}}`
)

func (cfg *MavenConfig) singleRequest(point *Point, pomPath string) {
	params := map[string]string{
		"maven2.groupId":      strings.TrimSpace(point.GroupId),
		"maven2.artifactId":   strings.TrimSpace(point.ArtifactId),
		"maven2.version":      strings.TrimSpace(point.Version),
		"maven2.generate-pom": "false",
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range params {
		writer.WriteField(k, v)
	}

	point.Extension = "pom"
	cfg.removeComponent(point)

	cfg.uploadSinglePom(pomPath, writer)

	e := writer.Close()
	failOnError(e, "close body buffer error")

	cfg.httpClientRequest(body, writer)
}

func (cfg *MavenConfig) request(point *Point, jarPath string) {
	params := map[string]string{
		"maven2.groupId":      strings.TrimSpace(point.GroupId),
		"maven2.artifactId":   strings.TrimSpace(point.ArtifactId),
		"maven2.version":      strings.TrimSpace(point.Version),
		"maven2.generate-pom": "false",
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range params {
		writer.WriteField(k, v)
	}

	cfg.removeComponent(point)

	cfg.uploadJar(jarPath, writer)
	cfg.uploadPom(jarPath, writer)
	cfg.uploadSource(jarPath, writer)

	e := writer.Close()
	failOnError(e, "close body buffer error")

	cfg.httpClientRequest(body, writer)
}

func (cfg *MavenConfig) httpClientRequest(body *bytes.Buffer, writer *multipart.Writer) {
	urlStr := cfg.parseStr(urlStrTmp, &map[string]string{
		"repo": cfg.RepoName,
		"addr": cfg.MavenServerHost,
	})
	req, e := http.NewRequest("POST", urlStr, body)
	failOnError(e, "http request failed")
	req.SetBasicAuth(cfg.Username, cfg.Secret)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	//client := &http.Client{}
	response, e := http.DefaultClient.Do(req)
	defer response.Body.Close()
	failOnError(e, "http request failed")
	//response.Write(os.Stdout)
	buf := &bytes.Buffer{}
	e = response.Write(buf)
	failOnError(e, "delete response write failed")
	*cfg.Msgch <- buf.String()
}

func (cfg *MavenConfig) removeComponent(point *Point) {
	searchUrl := cfg.parseStr(searchUrlStrTmp, &map[string]string{
		"g":    point.GroupId,
		"a":    point.ArtifactId,
		"v":    point.Version,
		"ext":  point.Extension,
		"repo": cfg.RepoName,
		"addr": cfg.MavenServerHost,
	})
	// fmt.Println(searchUrl)
	resp, e := http.Get(searchUrl)
	if resp == nil {
		return
	}
	defer resp.Body.Close()
	failOnError(e, "search response get failed")
	//body := &bytes.Buffer{}
	searchResopnseBuf, e := ioutil.ReadAll(resp.Body)
	failOnError(e, "search response read failed")

	searchResult := &SearchResult{}
	e = json.Unmarshal(searchResopnseBuf, searchResult)
	failOnError(e, "search response read failed")
	if len(searchResult.Items) > 0 && len(searchResult.Items[0].Id) > 0 {
		deleteUrl := cfg.parseStr(deleteUrlStrTmp, &map[string]string{
			"id":   searchResult.Items[0].Id,
			"addr": cfg.MavenServerHost,
		})
		delRequest, e := http.NewRequest("DELETE", deleteUrl, nil)
		failOnError(e, "construct delete url failed")
		delRequest.SetBasicAuth(cfg.Username, cfg.Secret)
		delResponse, e := http.DefaultClient.Do(delRequest)
		if delResponse == nil {
			return
		}
		defer delResponse.Body.Close()
		failOnError(e, "delete request back failed")
		//os.Stdout.WriteString("DELETE COMPONENT...\n")
		//delResponse.Write(os.Stdout)
		*cfg.Msgch <- "DELETE COMPONENT..."
		buf := &bytes.Buffer{}
		e = delResponse.Write(buf)
		failOnError(e, "delete response write failed")
		*cfg.Msgch <- buf.String()

		/*deleteResopnseBuf, e := ioutil.ReadAll(delResponse.Body)
		failOnError(e, "read delete back failed")
		fmt.Println(delResponse.StatusCode)
		fmt.Println(delResponse.Header)
		fmt.Println(string(deleteResopnseBuf))*/
	}
}

func (cfg *MavenConfig) parseStr(templateStr string, data *map[string]string) string {
	exp, e := template.New(time.Now().String()).Parse(templateStr)
	failOnError(e, "tempate load failed")
	buf := &bytes.Buffer{}
	(*data)["base"] = cfg.BaseContext
	e = exp.Execute(buf, *data)
	failOnError(e, "tempate execute failed")
	return buf.String()
}

func (cfg *MavenConfig) uploadJar(jarPath string, writer *multipart.Writer) {
	jarFile, e := os.Open(jarPath)
	failOnError(e, "open jar file error")
	defer jarFile.Close()
	part, e := writer.CreateFormFile("maven2.asset1", jarPath)
	failOnError(e, "create form file header error")
	_, e = io.Copy(part, jarFile)
	failOnError(e, "copy jar file to form-data error")

	writer.WriteField("maven2.asset1.extension", "jar")
}

func (cfg *MavenConfig) uploadSinglePom(pomPath string, writer *multipart.Writer) {
	pomFile, e := os.Open(pomPath)
	failOnError(e, "open pom file error")
	defer pomFile.Close()
	part, e := writer.CreateFormFile("maven2.asset1", pomPath)
	failOnError(e, "create form file header error")
	_, e = io.Copy(part, pomFile)
	failOnError(e, "copy pom file to form-data error")

	//writer.WriteField("maven2.asset2.classifier", "sources")
	writer.WriteField("maven2.asset1.extension", "pom")
}

func (cfg *MavenConfig) uploadPom(jarPath string, writer *multipart.Writer) {
	pomFilename := strings.Replace(jarPath, ".jar", ".pom", 1)
	pomFile, e := os.Open(pomFilename)
	failOnError(e, "open pom file error")
	defer pomFile.Close()
	part, e := writer.CreateFormFile("maven2.asset2", pomFilename)
	failOnError(e, "create form file header error")
	_, e = io.Copy(part, pomFile)
	failOnError(e, "copy pom file to form-data error")

	//writer.WriteField("maven2.asset2.classifier", "sources")
	writer.WriteField("maven2.asset2.extension", "pom")
}

func (cfg *MavenConfig) uploadSource(jarPath string, writer *multipart.Writer) {
	sourceFilename := strings.Replace(jarPath, ".jar", "-sources.jar", 1)
	if _, e := os.Stat(sourceFilename); e == nil {
		sourceFile, e := os.Open(sourceFilename)
		failOnError(e, "open source file error")
		defer sourceFile.Close()

		part, e := writer.CreateFormFile("maven2.asset3", sourceFilename)
		failOnError(e, "create form file2 header error")
		_, e = io.Copy(part, sourceFile)
		failOnError(e, "copy source file to form-data error")

		writer.WriteField("maven2.asset3.classifier", "sources")
		writer.WriteField("maven2.asset3.extension", "jar")
	}
}

func jarConfigWelcome(w http.ResponseWriter, _ *http.Request, data *MavenConfig) {
	if tmp, e := template.ParseFiles("web/pages/jar_upload.html"); e == nil {
		e := tmp.Execute(w, data)
		FailOnError(e, "template render error")
	} else {
		RenderError(w, e.Error(), http.StatusInternalServerError)
	}
}

func jarHandler(w http.ResponseWriter, r *http.Request, msgch *chan string) {
	mvnCfg := &MavenConfig{
		BaseContext:     r.PostFormValue("BaseContext"),
		LocalRepoDir:    r.PostFormValue("LocalRepoDir"),
		MavenServerHost: r.PostFormValue("MavenServerHost"),
		RepoName:        r.PostFormValue("RepoName"),
		Username:        r.PostFormValue("Username"),
		Secret:          r.PostFormValue("Secret"),
		Msgch:           msgch,
	}
	targetDir := mvnCfg.LocalRepoDir
	fileInfo, e := os.Stat(targetDir)
	failOnError(e, "")
	if !fileInfo.IsDir() {
		w.Write([]byte("本地仓库地址需要是一个目录"))
		return
	}

	go mvnCfg.execJarUpload()
}

func UploadJarHanler(msgch *chan string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		log.Println(method)
		switch method {
		case "GET":
			data := &MavenConfig{
				BaseContext:     "/service/rest",
				LocalRepoDir:    "D:\\.m2",
				MavenServerHost: "localhost:8081",
				RepoName:        "maven-releases",
				Username:        "admin",
				Secret:          "admin123",
				ReqHost:         r.Host,
			}

			if _, err := os.Stat("app.json"); err == nil {
				buf, _ := ioutil.ReadFile("app.json")

				if err = json.Unmarshal(buf, data); err != nil {
					log.Println("app.json parse failed")
				}
				data.ReqHost = r.Host
			}
			/*
						data := &MavenConfig{
					BaseContext:     "/nexus/service/rest",
					LocalRepoDir:    "D:\\.m2",
					MavenServerHost: "172.1.1.16:8081",
					RepoName:        "maven-releases",
					Username:        "Efy",
					Secret:          "oookkk",
					ReqHost:         r.Host,
				}


			*/
			jarConfigWelcome(w, r, data)
		case "POST":
			jarHandler(w, r, msgch)
		}
	})
}
