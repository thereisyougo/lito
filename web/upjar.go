package web

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
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
)

type PomPoint struct {
	Project    xml.Name    `xml:"project"`
	GroupId    string      `xml:"groupId"`
	ArtifactId string      `xml:"artifactId"`
	Version    string      `xml:"version"`
	Parent     ParentPoint `xml:"parent"`
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
	LocalRepoDir, MavenServerHost, RepoName, Username, Secret, ReqHost string
	Msgch                                                              *chan string
}

func verify(fileName string) bool  {
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
		lowName := strings.ToLower(info.Name())
		if !info.IsDir() && strings.HasSuffix(lowName, ".jar") &&
			!strings.HasSuffix(lowName, "-sources.jar") &&
			!strings.HasSuffix(lowName, "-snapshot.jar") &&
			!strings.HasSuffix(lowName, "-javadoc.jar") {

			if !verify(path) {
				fmt.Println("jar文件sha1校验失败")
				return nil
			}

			pomFilename := strings.Replace(path, ".jar", ".pom", 1)
			if _, e := os.Stat(pomFilename); os.IsNotExist(e) {
				return nil
			}

			buf, e := ioutil.ReadFile(pomFilename)
			failOnError(e, "read file error")

			point := &PomPoint{}
			e = xml.Unmarshal(buf, point)
			failOnError(e, "read file error")

			g, a, v := "", "", ""
			if len(point.ArtifactId) == 0 {
				fmt.Println("pom.xml artifactId is empty")
				return nil
			} else {
				g, a, v = point.GroupId, point.ArtifactId, point.Version
			}
			if len(point.GroupId) == 0 {
				g = point.Parent.GroupId
			}
			if len(point.Version) == 0 {
				v = point.Parent.Version
			}

			if len(g) != 0 && len(a) != 0 && len(v) != 0 {
				cfg.request(g, a, v, path)
			}
			count++
		}
		return nil
	})

	*cfg.Msgch <- fmt.Sprint("count: ", count)
}

const (
	urlStrTmp       string = `http://{{.addr}}/service/rest/v1/components?repository={{.repo}}`
	searchUrlStrTmp string = `http://{{.addr}}/service/rest/v1/search?repository={{.repo}}&maven.groupId={{.g}}&maven.artifactId={{.a}}&maven.baseVersion={{.v}}&maven.extension=jar`
	deleteUrlStrTmp string = `http://{{.addr}}/service/rest/v1/components/{{.id}}`
)

func (cfg *MavenConfig) request(g, a, v, jarPath string) {
	params := map[string]string{
		"maven2.groupId":      strings.TrimSpace(g),
		"maven2.artifactId":   strings.TrimSpace(a),
		"maven2.version":      strings.TrimSpace(v),
		"maven2.generate-pom": "false",
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range params {
		writer.WriteField(k, v)
	}

	cfg.removeComponent(g, a, v)

	cfg.uploadJar(jarPath, writer)
	cfg.uploadPom(jarPath, writer)
	cfg.uploadSource(jarPath, writer)

	e := writer.Close()
	failOnError(e, "close body buffer error")

	urlStr := parseStr(urlStrTmp, &map[string]string{
		"repo": cfg.RepoName,
		"addr": cfg.MavenServerHost,
	})
	req, e := http.NewRequest("POST", urlStr, body)
	req.SetBasicAuth(cfg.Username, cfg.Secret)
	failOnError(e, "http request failed")
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

func (cfg *MavenConfig) removeComponent(groupid, artifactid, version string) {
	searchUrl := parseStr(searchUrlStrTmp, &map[string]string{
		"g":    groupid,
		"a":    artifactid,
		"v":    version,
		"repo": cfg.RepoName,
		"addr": cfg.MavenServerHost,
	})
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
		deleteUrl := parseStr(deleteUrlStrTmp, &map[string]string{
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

func parseStr(templateStr string, data *map[string]string) string {
	exp, e := template.New(time.Now().String()).Parse(templateStr)
	failOnError(e, "tempate load failed")
	buf := &bytes.Buffer{}
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

func jarConfigWelcome(w http.ResponseWriter, r *http.Request, data *MavenConfig) {
	if tmp, e := template.ParseFiles("web/pages/jar_upload.html"); e == nil {
		e := tmp.Execute(w, data)
		FailOnError(e, "template render error")
	} else {
		RenderError(w, e.Error(), http.StatusInternalServerError)
	}
}

func jarHandler(w http.ResponseWriter, r *http.Request, msgch *chan string) {
	mvnCfg := &MavenConfig{
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
				LocalRepoDir:    "D:\\.m2",
				MavenServerHost: "localhost:8081",
				RepoName:        "maven-releases",
				Username:        "admin",
				Secret:          "admin123",
				ReqHost:         r.Host,
			}
			jarConfigWelcome(w, r, data)
		case "POST":
			jarHandler(w, r, msgch)
		}
	});
}
