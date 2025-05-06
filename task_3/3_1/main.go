package main

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
	deleteByAge()
}

type Student struct {
	Id    int
	Name  string
	Age   int
	Garde string
}

func insert() {
	student := Student{Name: "王五", Age: 16, Garde: "高中一年"}
	res := DB.Create(&student)
	if res.Error != nil {
		panic("插入数据时异常：" + res.Error.Error())
	}
	fmt.Println("pk_id", "name", "age", "garde", student.Id, student.Name, student.Age, student.Garde)
	fmt.Println("插入数据条数", res.RowsAffected)
}

func selectByAge() {
	student := Student{}
	DB.First(&student)                 // SELECT * FROM `students` ORDER BY `students`.`id` LIMIT 1
	fmt.Println("name:", student.Name) // name: 张三
	DB.Take(&student)                  // SELECT * FROM `students` WHERE `students`.`id` = 1 LIMIT 1
	fmt.Println("name:", student.Name) // name: 张三
	DB.Last(&student)                  // SELECT * FROM `students` WHERE `students`.`id` = 1 ORDER BY `students`.`id` DESC LIMIT 1
	fmt.Println("name:", student.Name) // name: 张三
	DB.Find(&student)                  // SELECT * FROM `students` WHERE `students`.`id` = 1
	fmt.Println("name:", student.Name) // name: 张三
	DB.Limit(2).Find(&student)         // SELECT * FROM `students` WHERE `students`.`id` = 1 LIMIT 2
	fmt.Println("name:", student.Name) // name: 张三

	s1 := Student{}
	DB.First(&s1, 10)             // SELECT * FROM `students` WHERE `students`.`id` = 10 ORDER BY `students`.`id` LIMIT 1
	fmt.Println("name:", s1.Name) // name:
	DB.Find(&s1, 2)               // SELECT * FROM `students` WHERE `students`.`id` = 2
	fmt.Println("name:", s1.Name) // name: 李四
	DB.Find(&s1, []int{1, 2, 3})  //  SELECT * FROM `students` WHERE `students`.`id` IN (1,2,3)
	fmt.Println("name:", s1.Name) // name: 张三
	DB.First(&s1, "id = ?", 2)    // SELECT * FROM `students` WHERE id = 2 AND `students`.`id` = 1 ORDER BY `students`.`id` LIMIT 1
	fmt.Println("name:", s1.Name) // name: 张三

	s2 := Student{}
	DB.Model(Student{}).Where("id = ?", 2).First(&s2) //  SELECT * FROM `students` WHERE id = 2 ORDER BY `students`.`id` LIMIT 1
	fmt.Println("name:", s1.Name)                     // name: 李四

	var ss []Student
	DB.Find(&ss)                 // SELECT * FROM `students`
	fmt.Println("len:", len(ss)) // len: 3

	var ss1 []Student
	DB.Where("name = ?", "王五").Find(&ss1) // SELECT * FROM `students` WHERE name = '王五'
	fmt.Println("len:", len(ss1))         // len: 1
	fmt.Println("name:", ss1[0].Name)     // name: 王五

	DB.Where("name <> ?", "王五").Find(&ss1) // SELECT * FROM `students` WHERE name <> '王五'
	fmt.Println("len:", len(ss1))          // len: 2
	fmt.Println("name:", ss1[0].Name)      // name: 张三

	DB.Where("age > ?", 1).Find(&ss1) // SELECT * FROM `students` WHERE age > 1
	fmt.Println("len:", len(ss1))     // len: 3
	fmt.Println("name:", ss1[0].Name) // name: 张三

	DB.Where("name LIKE ?", "%三%").Find(&ss1) // SELECT * FROM `students` WHERE name LIKE '%三%'
	fmt.Println("len:", len(ss1))             // len: 1
	fmt.Println("name:", ss1[0].Name)         // name: 张三

	var res []Student
	DB.Where("age > ?", 15).Find(&res) //  SELECT * FROM `students` WHERE age > 15
	fmt.Println("len:", len(res))
	for i := range res {
		fmt.Println("name:", res[i].Name)
		// name: 张三
		// name: 李四
		// name: 王五
	}
}

func updateByName() {
	//s := Student{}
	//DB.First(&s)
	//s.Name = "张三1"
	//s.Age = 19
	//s.Garde = "高中四年"
	//DB.Save(&s) // UPDATE `students` SET `name`='张三1',`age`=19,`garde`='高中四年' WHERE `id` = 1
	//fmt.Println("Name:", s.Name)
	//fmt.Println("Age:", s.Age)
	//fmt.Println("Garde:", s.Garde)

	//DB.Save(&Student{Name: "<UNK>", Age: 16, Garde: "<UNK>"})        // INSERT INTO `students` (`name`,`age`,`garde`) VALUES ('<UNK>1',16,'<UNK>')
	//DB.Save(&Student{Id: 5, Name: "<UNK>", Age: 16, Garde: "<UNK>"}) // UPDATE `students` SET `name`='<UNK>',`age`=16,`garde`='<UNK>' WHERE `id` = 5
	// 不要将 Save 和 Model一同使用, 这是 未定义的行为。
	// 编写SQL语句将 students 表中姓名为 "张三" 的学生年级更新为 "四年级"。
	//DB.Model(&Student{}).Where("id = ?", 5).Update("name", "哈哈哈") //  UPDATE `students` SET `name`='哈哈哈' WHERE id = 5

	// 批量更新 UPDATE `students` SET `age`=30,`name`='hello' WHERE id IN (4,5)
	DB.Table("students").Where("id IN ?", []int{4, 5}).Updates(map[string]interface{}{"name": "hello", "age": 30})
	// UPDATE `students` SET `garde`='博士' WHERE id IN (4,5)
	DB.Model(&Student{}).Where("id IN ?", []int{4, 5}).Updates(Student{Garde: "博士"})
}

func deleteByAge() {
	s := Student{}
	DB.Delete(&s) //  [rows:0] DELETE FROM `students`

	s.Id = 5
	DB.Delete(&s) //  [rows:1] DELETE FROM `students` WHERE `students`.`id` = 5
}
