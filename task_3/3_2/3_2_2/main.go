package main

import (
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql" // 必须添加这个空白导入
	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB

func init() {
	dsn := "root:123456789@tcp(127.0.0.1:3306)/td-homework?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	//defer func(db *sqlx.DB) {
	//	err := db.Close()
	//	if err != nil {
	//		panic(err)
	//	}
	//}(db)
	// 配置连接池
	db.SetMaxOpenConns(20)           // 最大打开连接数
	db.SetMaxIdleConns(10)           // 最大空闲连接数
	db.SetConnMaxLifetime(time.Hour) // 连接最大存活时间
	// 验证连接是否有效
	if err := db.Ping(); err != nil {
		log.Fatalf("无法ping通数据库: %v", err)
	}
	fmt.Println("成功连接到MySQL数据库")
	DB = db

}

func main() {
	example1()
}

/*
假设有一个 books 表，包含字段 id 、 title 、 author 、 price 。
要求 ：
定义一个 Book 结构体，包含与 books 表对应的字段。
编写Go代码，使用Sqlx执行一个复杂的查询，例如查询价格大于 50 元的书籍，并将结果映射到 Book 结构体切片中，确保类型安全。
*/

type Books struct {
	Id     int    `db:"id"`
	Title  string `db:"title"`
	Author string `db:"author"`
	Price  int    `db:"price"`
}

func getBooks() ([]Books, error) {
	var books []Books
	err := DB.Select(&books, "select id,title,author,price from books where price > ?", 50)
	if err != nil {
		return nil, err
	}
	return books, nil
}
func example1() {
	books, err := getBooks()
	if err != nil {
		log.Fatal(err)
	}
	for _, book := range books {
		fmt.Println(book.Title, book.Author, book.Price)
	}
}
