package main

import (
	"errors"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Transaction 事务操作

var DB *gorm.DB

func init() {
	mysqlLogger := logger.Default.LogMode(logger.Info)
	dsn := "root:123456789@tcp(127.0.0.1:3306)/td-homework?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: mysqlLogger,
	})
	if err != nil {
		panic("数据库连接失败：" + err.Error())
	}

	fmt.Println("数据库连接成功，db conn:", db)
	DB = db
}

/*
题目2：事务语句
	假设有两个表： accounts 表（包含字段 id 主键， balance 账户余额）和 transactions 表（包含字段 id 主键， from_account_id 转出账户ID， to_account_id 转入账户ID， amount 转账金额）。
要求 ：
	编写一个事务，实现从账户 A 向账户 B 转账 100 元的操作。在事务中，需要先检查账户 A 的余额是否足够，如果足够则从账户 A 扣除 100 元，向账户 B 增加 100 元，并在 transactions 表中记录该笔转账信息。如果余额不足，则回滚事务。
*/

func main() {
	// 示例：从账户1向账户2转账100元
	err := transferMoney(DB, 1, 2, 1000.0)
	if err != nil {
		fmt.Println("转账失败:", err)
	} else {
		fmt.Println("转账成功")
	}
}

type Account struct {
	ID      uint
	Balance float64
}

type Transaction struct {
	ID            uint
	FromAccountID uint
	ToAccountID   uint
	Amount        float64
}

// 转账
func transferMoney(db *gorm.DB, fromAccountID, toAccountID uint, amount float64) error {
	// 开始事务
	tx := DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback() // 回滚
		}
	}()

	if tx.Error != nil {
		return tx.Error
	}

	// 检查余额是否足够
	var fromAccount Account
	if err := tx.First(&fromAccount, fromAccountID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("转出账户不存在")
		}
	}
	if fromAccount.Balance < amount {
		return fmt.Errorf("余额不足")
	}

	var toAccount Account
	if err := tx.First(&toAccount, toAccountID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("转入账户不存在")
		}
	}

	if err := tx.Model(&fromAccount).Update("balance",
		gorm.Expr("balance - ?", amount)).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Model(&toAccount).Update("balance",
		gorm.Expr("balance + ?", amount)).Error; err != nil {
		tx.Rollback()
		return err
	}

	transaction := Transaction{
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Amount:        amount,
	}
	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
