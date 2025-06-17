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
	example2()
}

/*
题目1：使用SQL扩展库进行查询
	假设你已经使用Sqlx连接到一个数据库，并且有一个 employees 表，包含字段 id 、 name 、 department 、 salary 。
要求 ：
	编写Go代码，使用Sqlx查询 employees 表中所有部门为 "技术部" 的员工信息，并将结果映射到一个自定义的 Employee 结构体切片中。
	编写Go代码，使用Sqlx查询 employees 表中工资最高的员工信息，并将结果映射到一个 Employee 结构体中。
*/

// Employee 结构体映射employees表
type Employee struct {
	ID         int    `db:"id"`
	Name       string `db:"name"`
	Department string `db:"department"`
	Salary     int    `db:"salary"`
}

func getTechEmployees(db *sqlx.DB) ([]Employee, error) {
	var employees []Employee

	// 使用Select查询多行记录，自动映射到结构体切片
	err := db.Select(&employees,
		"SELECT id, name, department, salary FROM employees WHERE department = ?",
		"技术部")

	if err != nil {
		return nil, fmt.Errorf("查询技术部员工失败: %v", err)
	}

	return employees, nil
}

// 使用示例
func example1() {
	employees, err := getTechEmployees(DB)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println("技术部员工:")
	for _, emp := range employees {
		fmt.Printf("ID: %d, 姓名: %s, 薪资: %d\n", emp.ID, emp.Name, emp.Salary)
	}
}

func getHighestPaidEmployee(db *sqlx.DB) (*Employee, error) {
	var employee Employee

	// 使用Get查询单条记录
	err := db.Get(&employee,
		"SELECT id, name, department, salary FROM employees ORDER BY salary DESC LIMIT 1")

	if err != nil {
		return nil, fmt.Errorf("查询最高薪资员工失败: %v", err)
	}

	return &employee, nil
}

// 使用示例
func example2() {
	emp, err := getHighestPaidEmployee(DB)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Printf("\n薪资最高的员工: %s, 部门: %s, 薪资: %d\n",
		emp.Name, emp.Department, emp.Salary)
}
