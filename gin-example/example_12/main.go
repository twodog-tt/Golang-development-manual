package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Booking 包含绑定和验证的数据。
type Booking struct {
	CheckIn  time.Time `form:"check_in" binding:"required,bookabledate" time_format:"2006-01-02"`
	CheckOut time.Time `form:"check_out" binding:"required,gtfield=CheckIn,bookabledate" time_format:"2006-01-02"`

	/*
				form:"check_in"：绑定查询参数中的 check_in 字段
			    binding:"required,bookabledate"：验证规则（必填 + 自定义验证）
		        time_format:"2006-01-02"：指定时间格式（Go 的参考时间格式）
				gtfield=CheckIn：验证 CheckOut 必须大于 CheckIn
	*/
}

var bookableDate validator.Func = func(fl validator.FieldLevel) bool {
	date, ok := fl.Field().Interface().(time.Time)
	if ok {
		today := time.Now()
		if today.After(date) {
			return false // 日期在今天之前则验证失败
		}
	}
	return true
}

func main() {
	route := gin.Default()

	// 注册自定义验证器
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("bookabledate", bookableDate)
	}

	route.GET("/bookable", getBookable)
	route.Run(":8085")
}

func getBookable(c *gin.Context) {
	var b Booking
	if err := c.ShouldBindWith(&b, binding.Query); err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Booking dates are valid!"})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

/*
curl "localhost:8085/bookable?check_in=2018-04-16&check_out=2018-04-17"
{"error":"Key: 'Booking.CheckIn' Error:Field validation for 'CheckIn' failed on the 'bookabledate' tag\nKey: 'Booking.CheckOut' Error:Field validation for 'CheckOut' failed on the 'bookabledate' tag"}

curl "localhost:8085/bookable?check_in=2025-05-08&check_out=2025-05-09"
{"message":"Booking dates are valid!"}
*/
