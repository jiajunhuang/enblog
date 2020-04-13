package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/gin-contrib/sentry"
	"github.com/gin-gonic/gin"
	redis "github.com/go-redis/redis/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/russross/blackfriday/v2"
)

const (
	zsetKey = "enblogtopn"
)

var (
	filenameRegex = regexp.MustCompile(`(\d{4}_\d{2}_\d{2})-.+\..+`)
	articles      = LoadMDs("articles")

	redisClient *redis.Client

	// ErrNotFound means article not found
	ErrNotFound = errors.New("Article Not Found")
	// ErrFailedToLoad failed to load article
	ErrFailedToLoad = errors.New("Failed To Load Article")

	// Prometheus
	totalRequests = promauto.NewCounter(prometheus.CounterOpts{Name: "total_requests_total"})
)

// InitSentry 初始化sentry
func InitSentry() {
	raven.SetDSN(os.Getenv("SENTRY_DSN"))
}

// InitializeRedis 初始化Redis
func InitializeRedis() {
	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Fatalf("failed to connect to redis db: %s", err)
	}

	// Create client as usually.
	redisClient = redis.NewClient(opt)
}

// Article 就是文章
type Article struct {
	Title       string    `json:"title"`
	Date        string    `json:"date_str"`
	Filename    string    `json:"file_name"`
	DirName     string    `json:"dir_name"`
	PubDate     time.Time `json:"-"`
	Description string    `json:"description"`
}

// Articles 文章列表
type Articles []Article

func (a Articles) Len() int      { return len(a) }
func (a Articles) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a Articles) Less(i, j int) bool {
	v := strings.Compare(a[i].Date, a[j].Date)
	if v <= 0 {
		return true
	}

	return false
}

// RandomN return n articles by random
func (a Articles) RandomN(n int) Articles {
	if n <= 0 {
		return nil
	}

	if len(a) < n {
		return nil
	}

	length := len(a)

	pos := rand.Intn(length - n)
	return a[pos : pos+n]
}

func isBlogApp(c *gin.Context) bool {
	ua := c.GetHeader("User-Agent")
	if strings.HasPrefix(ua, "BlogApp/") {
		return true
	}

	return false
}

func getFilePath(path string) string {
	suffix := ".html"
	if strings.HasSuffix(path, suffix) {
		path = path[:len(path)-len(suffix)]
	}
	return "./" + path
}

// ReadDesc 把简介读出来
func ReadDesc(path string) string {
	path = getFilePath(path)

	file, err := os.Open(path)
	if err != nil {
		log.Printf("failed to read file(%s): %s", path, err)
		return ""
	}
	reader := bufio.NewReader(file)
	reader.ReadLine() // 忽略第一行(标题)
	reader.ReadLine() // 忽略第二行(空行)
	desc := ""
	for i := 0; i < 3; i++ {
		line, _, err := reader.ReadLine()
		if err != nil && err != io.EOF {
			log.Printf("failed to read desc of file(%s): %s", path, err)
			continue
		}
		desc += string(line)
	}

	trimChars := "\n，。：,.:"
	return strings.TrimRight(strings.TrimLeft(desc, trimChars), trimChars) + "..."
}

// ReadTitle 把标题读出来
func ReadTitle(path string) string {
	path = getFilePath(path)

	file, err := os.Open(path)
	if err != nil {
		log.Printf("failed to read file(%s): %s", path, err)
		return ""
	}
	line, _, err := bufio.NewReader(file).ReadLine()
	if err != nil {
		log.Printf("failed to read title of file(%s): %s", path, err)
		return ""
	}
	title := strings.Replace(string(line), "# ", "", -1)

	return title
}

// VisitedArticle is for remember which article had been visited
type VisitedArticle struct {
	URLPath string `json:"url_path"`
	Title   string `json:"title"`
}

func genVisited(urlPath, subTitle string) (string, error) {
	title := ReadTitle(urlPath)
	if title == "" {
		return "", ErrNotFound
	}

	if subTitle != "" {
		title += " - " + subTitle
	}

	visited := VisitedArticle{URLPath: urlPath, Title: title}
	b, err := json.Marshal(visited)
	if err != nil {
		return "", ErrFailedToLoad
	}

	return string(b), nil
}

func getTopVisited(n int) []VisitedArticle {
	visitedArticles := []VisitedArticle{}

	articles, err := redisClient.ZRevRangeByScore(zsetKey, &redis.ZRangeBy{
		Min: "-inf", Max: "+inf", Offset: 0, Count: int64(n),
	}).Result()
	if err != nil {
		log.Printf("failed to get top %d visited articles: %s", n, err)
		return nil
	}

	for _, article := range articles {
		var va VisitedArticle
		if err := json.Unmarshal([]byte(article), &va); err != nil {
			log.Printf("failed to unmarshal article: %s", err)
			continue
		}

		visitedArticles = append(visitedArticles, va)
	}

	return visitedArticles
}

// LoadArticle 把文章的元信息读出来
func LoadArticle(dirname, filename string) *Article {
	match := filenameRegex.FindStringSubmatch(filename)
	if len(match) != 2 {
		return nil
	}

	dateString := strings.Replace(match[1], "_", "-", -1)
	filepath := fmt.Sprintf("./%s/%s", dirname, filename)
	title := ReadTitle(filepath)
	pubDate, err := time.Parse("2006-01-02", dateString)
	if err != nil {
		log.Panicf("failed to parse date: %s", err)
	}
	desc := ReadDesc(filepath)

	return &Article{
		Title:       title,
		Date:        dateString,
		Filename:    filename,
		DirName:     dirname,
		PubDate:     pubDate,
		Description: desc,
	}
}

// LoadMDs 读取给定目录中的所有markdown文章
func LoadMDs(dirname string) Articles {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatalf("failed to read dir(%s): %s", dirname, err)
		return nil
	}

	var articles Articles
	for _, file := range files {
		filename := file.Name()
		if article := LoadArticle(dirname, filename); article != nil {
			articles = append(articles, *article)
		}
	}

	sort.Sort(sort.Reverse(articles))

	return articles
}

// IndexHandler 首页
func IndexHandler(c *gin.Context) {
	topArticles := getTopVisited(15)
	indexLength := len(articles)
	if indexLength >= 100 {
		indexLength = 100
	}
	c.HTML(
		http.StatusOK, "index.html", gin.H{
			"isBlogApp":   isBlogApp(c),
			"articles":    articles[:indexLength],
			"totalCount":  len(articles),
			"keywords":    "Golang,Python,Distributed System,High Concurrency,Haskell,C,MicroService,Code Analysis",
			"description": "Enjoy Programming~Distributed System,High Concurrency/Golang/Python/Haskell/C/MicroService/Code Analysis",
			"topArticles": topArticles,
		},
	)
}

// ArchiveHandler 全部文章
func ArchiveHandler(c *gin.Context) {
	c.HTML(
		http.StatusOK, "index.html", gin.H{
			"isBlogApp":   isBlogApp(c),
			"articles":    articles,
			"keywords":    "Golang,Python,Distributed System,High Concurrency,Haskell,C,MicroService,Code Analysis",
			"description": "Enjoy Programming~Distributed System,High Concurrency/Golang/Python/Haskell/C/MicroService/Code Analysis",
		},
	)
}

func renderArticle(c *gin.Context, status int, path string, subtitle string, randomN int) {
	path = getFilePath(path)
	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("failed to read file %s: %s", path, err)
		c.Redirect(http.StatusFound, "/404")
		return
	}

	content = blackfriday.Run(
		content,
		blackfriday.WithExtensions(blackfriday.CommonExtensions),
	)

	recommends := articles.RandomN(randomN)
	topArticles := getTopVisited(15)

	c.HTML(
		status, "article.html", gin.H{
			"isBlogApp":   isBlogApp(c),
			"content":     template.HTML(content),
			"title":       ReadTitle(path),
			"subtitle":    subtitle,
			"recommends":  recommends,
			"topArticles": topArticles,
		},
	)
}

func incrVisited(urlPath, subTitle string) {
	if visited, err := genVisited(urlPath, subTitle); err != nil {
		log.Printf("failed to gen visited: %s", err)
	} else {
		if _, err := redisClient.ZIncrBy(zsetKey, 1, visited).Result(); err != nil {
			log.Printf("failed to incr score of %s: %s", urlPath, err)
		}
	}
}

// PingPongHandler ping pong
func PingPongHandler(c *gin.Context) {
	c.JSON(http.StatusOK, nil)
}

// ArticleHandler 具体文章
func ArticleHandler(c *gin.Context) {
	urlPath := c.Request.URL.Path
	incrVisited(urlPath, "")

	renderArticle(c, http.StatusOK, urlPath, "", 15)
}

// TutorialPageHandler 教程index
func TutorialPageHandler(c *gin.Context) {
	renderArticle(c, http.StatusOK, "articles/tutorial.md", "", 0)
}

// AboutMeHandler 关于我
func AboutMeHandler(c *gin.Context) {
	renderArticle(c, http.StatusOK, "articles/aboutme.md", "", 0)
}

// FriendsHandler 友链
func FriendsHandler(c *gin.Context) {
	renderArticle(c, http.StatusOK, "articles/friends.md", "", 0)
}

// AppHandler App页面
func AppHandler(c *gin.Context) {
	renderArticle(c, http.StatusOK, "articles/app.md", "", 0)
}

// NotFoundHandler 404
func NotFoundHandler(c *gin.Context) {
	renderArticle(c, http.StatusOK, "articles/404.md", "", 20)
}

// RSSHandler RSS
func RSSHandler(c *gin.Context) {
	c.Header("Content-Type", "application/xml")
	c.HTML(
		http.StatusOK, "rss.html", gin.H{
			"isBlogApp": isBlogApp(c),
			"rssHeader": template.HTML(`<?xml version="1.0" encoding="UTF-8"?>`),
			"articles":  articles,
		},
	)
}

// SiteMapHandler sitemap
func SiteMapHandler(c *gin.Context) {
	c.Header("Content-Type", "application/xml")
	c.HTML(
		http.StatusOK, "sitemap.html", gin.H{
			"isBlogApp": isBlogApp(c),
			"rssHeader": template.HTML(`<?xml version="1.0" encoding="UTF-8"?>`),
			"articles":  articles,
		},
	)
}

// SearchHandler 搜索
func SearchHandler(c *gin.Context) {
	word := c.PostForm("search")

	c.Redirect(
		http.StatusFound,
		"https://www.google.com/search?q=site:blog.jiajunhuang.com "+word,
	)
}

func main() {
	InitSentry()
	InitializeRedis()

	r := gin.New()

	r.Use(gin.Logger())
	r.Use(sentry.Recovery(raven.DefaultClient, false))
	r.Use(func(c *gin.Context) {
		totalRequests.Inc()
	})

	r.LoadHTMLGlob("templates/*.html")
	r.Static("/static", "./static")
	//r.Static("/tutorial/:lang/img/", "./tutorial/:lang/img")  # 然而不支持
	//r.Static("/articles/img", "./articles/img")  # 然而有冲突
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.StaticFile("/robots.txt", "./static/robots.txt")
	r.StaticFile("/ads.txt", "./static/ads.txt")

	r.GET("/", IndexHandler)
	r.GET("/ping", PingPongHandler)
	r.GET("/404", NotFoundHandler)
	r.GET("/archive", ArchiveHandler)
	r.GET("/articles/:filepath", ArticleHandler)
	r.GET("/aboutme", AboutMeHandler)
	r.GET("/tutorial", TutorialPageHandler)
	r.GET("/friends", FriendsHandler)
	r.GET("/app", AppHandler)
	r.GET("/rss", RSSHandler)
	r.GET("/sitemap.xml", SiteMapHandler)
	r.POST("/search", SearchHandler)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.NoRoute(func(c *gin.Context) { c.Redirect(http.StatusFound, "/404") })

	r.Run("0.0.0.0:8089")
}
