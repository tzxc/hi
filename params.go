package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	gourl "net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const authRegexp = `^(.+):([^\s].+)`

type headerSlice []string

func (h *headerSlice) String() string {
	return fmt.Sprintf("%s", *h)
}

func (h *headerSlice) Set(value string) error {
	*h = append(*h, value)
	return nil
}

//Params hey测试参数
type Params struct {
	//接收参数
	m, d, D, A, T, a, host, o, x                                *string
	c, n, t, cpus                                               *int
	q                                                           *float64
	z                                                           *time.Duration
	h2, disableCompression, disableKeepAlives, disableRedirects *bool
	//构造的参数
	url, username, password string //目标网址
	proxyURL                *gourl.URL
	hs                      headerSlice
}

//InitArgs 从命令参数中加载和初始化参数
func (p *Params) InitArgs() {
	//fmt.Println("Init:", os.Args[1:])
	p.m = flag.String("m", "GET", "HTTP方法, 可选择GET, POST, PUT, DELETE, HEAD, OPTIONS.")
	p.d = flag.String("d", "", "HTTP请求body")
	p.D = flag.String("D", "", "从文件读取HTTP请求body.例如, /home/user/file.txt or ./file.txt.")
	p.A = flag.String("A", "", "HTTP Accept header.")
	p.T = flag.String("T", "text/html", `Content-type, 默认"text/html".`)
	p.a = flag.String("a", "", "Basic authentication, username:password.")
	p.host = flag.String("host", "", "HTTP Host header.")
	p.o = flag.String("o", "", "输出csv文件. 默认直接打印屏幕汇总.")
	p.c = flag.Int("c", 50, "并发请求数. 不能大于总请求数，默认50")
	p.n = flag.Int("n", 200, "总请求数. 默认200. 设定1表示直接返回测试目标的输出，而不进行负荷测试。")
	p.q = flag.Float64("q", 0, "每秒查询数(QPS). 默认无限制.")
	p.t = flag.Int("t", 20, "每个请求的超时时间. 默认20s, 设定0则不超时.")
	p.z = flag.Duration("z", 0, "持续时间. 持续时间到达则自动停止和退出程序,若指定本参数则自动忽略总请求数.例如: -z 10s -z 3m.")
	p.h2 = flag.Bool("h2", false, "开启HTTP/2.")
	p.cpus = flag.Int("cpus", runtime.NumCPU(), "使用的CPU核心数(默认使用"+strconv.Itoa(runtime.NumCPU())+"cores)")
	p.disableCompression = flag.Bool("disable-compression", false, "关闭传输压缩.")
	p.disableKeepAlives = flag.Bool("disable-keepalive", false, "关闭keep-alive, 阻止不同的请求对连接进行重用")
	p.disableRedirects = flag.Bool("disable-redirects", false, "Disable following of HTTP redirects")
	p.x = flag.String("x", "", "HTTP代理地址，如host:port.")
	flag.Var(&(p.hs), "H", "")
	flag.Parse()
	if flag.NArg() > 0 {
		p.url = flag.Args()[0]
	}
	//fmt.Println("Inited:", *p.T)
	//flag.Usage()
}

//Check 检查参数
func (p *Params) Check() error {
	//检查url
	if p.url == "" {
		return errors.New("必须指定目标URL")
	}
	//自动添加protocol scheme
	if !strings.Contains(p.url, "://") {
		p.url = "http://" + p.url
		fmt.Println("自动修正测试目标为:", p.url)
	}
	_, err := gourl.Parse(p.url)
	if err != nil {
		return err
	}

	//若有则检查验证basic auth
	if *p.a != "" {
		re := regexp.MustCompile(authRegexp)
		matches := re.FindStringSubmatch(*p.a)
		if len(matches) < 1 {
			return fmt.Errorf("-a 不能解析输入: %v", *p.a)
		}
		p.username, p.password = matches[1], matches[2]
	}

	if *p.o != "csv" && *p.a != "" {
		return errors.New("-o 非法格式;目前仅支持csv")
	}

	if *p.x != "" {
		p.proxyURL, err = gourl.Parse(*p.x)
		if err != nil {
			return err
		}
	}

	//若是单次测试则不检查-n -c -z
	if *p.n == 1 {
		return nil
	}

	if *p.z > 0 { //持续时间>0
		*p.n = math.MaxInt32
		if *p.c <= 0 {
			return errors.New("-c 不能小于1")
		}
	} else {
		if *p.n <= 0 || *p.c <= 0 {
			return errors.New("-n 和 -c 不能小于1")
		}

		if *p.n < *p.c {
			return errors.New("-n 指定值不能小于 -c")
		}
	}
	return nil
}
