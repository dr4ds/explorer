package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"unicode/utf16"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasttemplate"
	"golang.org/x/sys/windows"
)

func GetAllDisks() []string {
	bufSize := uint32(1024)
	buf := make([]uint16, bufSize)
	n, err := windows.GetLogicalDriveStrings(bufSize, &buf[0])
	if err != nil {
		panic(err)
	}

	decoded := utf16.Decode(buf[:n])
	splitted := strings.Split(string(decoded), string(0))
	return splitted[:len(splitted)-1]
}

func NotFound(ctx *fasthttp.RequestCtx) {
	ctx.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
}

func InternalServerError(ctx *fasthttp.RequestCtx, err interface{}) {
	if err != nil {
		fmt.Println(err)
	}
	ctx.Error(fasthttp.StatusMessage(fasthttp.StatusInternalServerError), fasthttp.StatusInternalServerError)
}

func SendDisks(ctx *fasthttp.RequestCtx) {
	disks := GetAllDisks()

	objects := make([]Object, 0)
	for _, disk := range disks {
		objects = append(objects, Object{Name: disk, IsDir: true})
	}
	SendObjects(ctx, objects, false)
}

type Data struct {
	Objects   []Object `json:"objects"`
	HasParent bool     `json:"has_parent"`
}

type Object struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

func SendObjects(ctx *fasthttp.RequestCtx, objects []Object, hasParent bool) {
	b, err := json.Marshal(Data{HasParent: hasParent, Objects: objects})
	if err != nil {
		InternalServerError(ctx, err)
		return
	}

	t := fasttemplate.New(httpTemplate, "{ ", " }")
	s := t.ExecuteString(map[string]interface{}{
		"data": base64.StdEncoding.EncodeToString(b),
	})

	ctx.SetContentType("text/html; charset=utf-8")

	ctx.WriteString(s)
}

func SendFiles(ctx *fasthttp.RequestCtx, files []os.FileInfo) {
	objects := make([]Object, 0)
	for _, file := range files {
		objects = append(objects, Object{Name: file.Name(), IsDir: file.IsDir()})
	}
	SendObjects(ctx, objects, true)
}

func GetContent(ctx *fasthttp.RequestCtx) {
	path, ok := ctx.UserValue("path").(string)
	if !ok {
		ctx.WriteString("invalid path")
		return
	}

	if path == "/" {
		SendDisks(ctx)
		return
	}

	path = path[1:]

	stat, err := os.Stat(path)
	if err != nil {
		switch err.(type) {
		case *os.PathError:
			NotFound(ctx)
			return
		default:
			InternalServerError(ctx, err)
			return
		}
	}

	if !stat.IsDir() {
		ctx.SendFile(path)
		return
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		InternalServerError(ctx, err)
		return
	}
	SendFiles(ctx, files)
}

var httpTemplate string

func init() {
	b, err := ioutil.ReadFile("./client/index.html")
	if err != nil {
		panic(err)
	}
	httpTemplate = string(b)
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	filtered := make([]string, 0)
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				filtered = append(filtered, ipnet.IP.String())
			}
		}
	}
	return filtered[len(filtered)-1]
}

func main() {
	r := router.New()
	r.GET("/*path", GetContent)

	fmt.Println(GetLocalIP() + ":8080")

	s := fasthttp.Server{LogAllErrors: false, Logger: nil}
	s.Handler = r.Handler
	panic(s.ListenAndServe(":8080"))
}
