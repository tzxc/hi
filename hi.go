// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command hey is an HTTP load generator.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"time"

	"aix/web/hey"
)

const (
	headerRegexp = `^([\w-]+):\s*(.+)`
	//authRegexp   = `^(.+):([^\s].+)`
	heyUA = "hey/0.0.1"
)

var usage = `用法: hey [options...] <url>

Options:
  -n  总请求数. 默认200. 设定1表示直接返回测试目标的输出，而不进行负荷测试。
  -c  并发请求数. 不能大于总请求数，默认50.
  -q  每秒查询数(QPS). 默认无限制.
  -z  持续时间. 持续时间到达则自动停止和退出程序,若指定本参数则自动忽略总请求数
      例如: -z 10s -z 3m.
  -o  输出csv文件. 默认直接打印屏幕汇总.
  -m  HTTP方法, 可选择GET, POST, PUT, DELETE, HEAD, OPTIONS.
  -T  Content-type, 默认"text/html".
  -H  定制HTTP header. 通过重复指定本参数设置多条.
      例如, -H "Accept: text/html" -H "Content-Type: application/xml" .
  -t  每个请求的超时时间. 默认20s, 设定0则不超时.
  -A  HTTP Accept header.
  -d  HTTP请求body.
  -D  从文件读取HTTP请求body.例如, /home/user/file.txt or ./file.txt.
  -a  Basic authentication, username:password.
  -x  HTTP代理地址，如host:port.
  -h2 开启HTTP/2.

  -host	HTTP Host header.

  -disable-compression  关闭传输压缩.
  -disable-keepalive    关闭keep-alive, 阻止不同的请求对连接进行重用
  -disable-redirects    Disable following of HTTP redirects
  -cpus                 使用的CPU核心数(默认使用%dcores)
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage, runtime.NumCPU()))
	}
	var p Params
	//flag.Parse()
	p.InitArgs()
	if err := p.Check(); err != nil {
		usageAndExit(err.Error())
	}

	method := strings.ToUpper(*p.m)
	// set content-type
	header := make(http.Header)
	header.Set("Content-Type", *p.T)
	// set any other additional repeatable headers
	for _, h := range p.hs {
		match, err := parseInputWithRegexp(h, headerRegexp)
		if err != nil {
			usageAndExit(err.Error())
		}
		header.Set(match[1], match[2])
	}

	if *p.A != "" {
		header.Set("Accept", *p.A)
	}

	var bodyAll []byte
	if *p.d != "" {
		bodyAll = []byte(*p.d)
	}
	if *p.D != "" {
		slurp, err := ioutil.ReadFile(*p.D)
		if err != nil {
			errAndExit(err.Error())
		}
		bodyAll = slurp
	}

	req, err := http.NewRequest(method, p.url, nil)
	if err != nil {
		usageAndExit(err.Error())
	}
	req.ContentLength = int64(len(bodyAll))
	if p.username != "" || p.password != "" {
		req.SetBasicAuth(p.username, p.password)
	}

	// set host header if set
	if *p.host != "" {
		req.Host = *p.host
	}

	ua := req.UserAgent()
	if ua == "" {
		ua = heyUA
	} else {
		ua += " " + heyUA
	}
	header.Set("User-Agent", ua)
	req.Header = header
	//fmt.Println("运行前:", header, *p.T)
	//若是单次测试
	if *p.n == 1 {
		s := &hey.Work{
			Request:            req,
			RequestBody:        bodyAll,
			Timeout:            *p.t,
			DisableCompression: *p.disableCompression,
			DisableKeepAlives:  *p.disableKeepAlives,
			DisableRedirects:   *p.disableRedirects,
			H2:                 *p.h2,
			ProxyAddr:          p.proxyURL,
			Output:             *p.o,
		}
		fmt.Println(s.RunSimple())
		os.Exit(0)
	}

	w := &hey.Work{
		Request:            req,
		RequestBody:        bodyAll,
		N:                  *p.n,
		C:                  *p.c,
		QPS:                *p.q,
		Timeout:            *p.t,
		DisableCompression: *p.disableCompression,
		DisableKeepAlives:  *p.disableKeepAlives,
		DisableRedirects:   *p.disableRedirects,
		H2:                 *p.h2,
		ProxyAddr:          p.proxyURL,
		Output:             *p.o,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		w.Stop()
	}()
	if *p.z > 0 {
		go func() {
			time.Sleep(*p.z)
			w.Stop()
		}()
	}
	w.Run()
}

//打印错误并退出
func errAndExit(msg string) {
	fmt.Fprintf(os.Stderr, msg)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

//打印用法并退出
func usageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func parseInputWithRegexp(input, regx string) ([]string, error) {
	re := regexp.MustCompile(regx)
	matches := re.FindStringSubmatch(input)
	if len(matches) < 1 {
		return nil, fmt.Errorf("不能解析输入: %v", input)
	}
	return matches, nil
}
