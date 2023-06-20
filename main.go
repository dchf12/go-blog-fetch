package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	_ "embed"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed url.txt
var baseURL string

type article struct {
	title string
	url   string
	date  string
	read  bool
}

// title, urlでUKになるSQLite３のDBを作成
const schema = `
CREATE TABLE IF NOT EXISTS articles (
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    date DATE NOT NULL,
    read BOOLEAN DEFAULT FALSE,
    UNIQUE (url, title)
);
`

// db connectionを保持
var db *sql.DB

//go:embed webhook.txt
var webhookURL string

func init() {
	// DBを開く
	var err error
	db, err = sql.Open("sqlite3", "blog.db")
	if err != nil {
		log.Fatal(err)
	}
	// SQLを実行
	_, err = db.Exec(schema)
	if err != nil {
		log.Fatal(err)
	}

	baseURL = strings.TrimSpace(baseURL)
}

func main() {
	// 金曜日だけ実行
	if time.Now().Weekday() != time.Friday {
		// すべての記事を取得
		fetchAllArticles()
	}

	// 記事のURLにアクセス
	var urls []string
	rows, err := db.Query("SELECT url FROM articles WHERE read = 0 ORDER BY date")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			log.Fatal(err)
		}
		urls = append(urls, url)
	}

	for _, url := range urls[:3] {
		// slackに通知
		if err := notifySlack(url); err != nil {
			log.Fatal(err)
		}
		// 記事を既読にする
		if err := markAsRead(url); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("finish")
}

func notifySlack(msg string) error {
	// slackに通知
	//json marshal
	payload, err := json.Marshal(map[string]string{
		"text": msg,
	})
	if err != nil {
		log.Fatal(err)
	}
	// POSTリクエストを送信
	webhookURL = strings.TrimSpace(webhookURL)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		log.Fatal("post error:", err)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error: status code", resp.StatusCode)
		return err
	}
	return nil
}
func markAsRead(url string) error {
	// トランザクションの開始
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
		return err
	}
	// トランザクションの終了
	defer tx.Rollback()
	// SQLの準備
	stmt, err := tx.Prepare("UPDATE articles SET read = 1 WHERE url = ?")
	if err != nil {
		log.Fatal(err)
		return err
	}
	// SQLの終了
	defer stmt.Close()
	// SQLの実行
	_, err = stmt.Exec(url)
	if err != nil {
		log.Fatal(err)
		return err
	}
	// トランザクションの終了
	if err = tx.Commit(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func fetchAllArticles() {
	resp, err := http.Get(baseURL)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error: status code", resp.StatusCode)
		return
	}
	defer resp.Body.Close()
	// HTMLをパース
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var articles []article
	// セレクタで指定した要素を取得
	doc.Find(".article-list").Each(func(i int, s *goquery.Selection) {
		//sの下にある全てのliタグを取得
		s.Find("li").Each(func(j int, s *goquery.Selection) {
			//href属性の値を取得
			href, _ := s.Find("a").Attr("href")
			//title属性の値を取得
			title, _ := s.Find("a").Attr("title")
			//class="date"の値を取得
			date := s.Find(".date").Text()
			// 2023.06.20をtime.Timeに変換
			t, err := time.Parse("2006.01.02", date)
			if err != nil {
				log.Fatal(err)
			}
			outputDate := t.Format("2006-01-02")
			// hrefから/articlesを削除
			path := strings.Replace(href, "/articles/", "", 1)
			// url join
			endpoint, err := url.JoinPath(baseURL, path)
			if err != nil {
				log.Fatal(err)
			}
			articles = append(articles, article{title: title, url: endpoint, date: outputDate})
		})
	})

	if err := saveAllArticles(articles); err != nil {
		log.Fatal(err)
	}
}
func saveAllArticles(articles []article) error {
	// articlesをDBに保存
	// トランザクションの開始
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
		return err
	}
	// トランザクションの終了
	defer tx.Rollback()
	// SQLの準備
	stmt, err := tx.Prepare("INSERT INTO articles (title, url, date) VALUES (?, ?, ?)")
	if err != nil {
		log.Fatal(err)
		return err
	}
	// SQLの終了
	defer stmt.Close()
	// SQLの実行
	for _, article := range articles {
		_, err := stmt.Exec(article.title, article.url, article.date)
		if err != nil {
			// 重複エラーをチェック
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			} else {
				log.Fatal("stmt.Exec: ", err)
			}
		}
	}
	// コミット
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}
