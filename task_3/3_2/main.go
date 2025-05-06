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
