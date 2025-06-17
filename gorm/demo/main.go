package main

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

/*
题目1：模型定义
假设你要开发一个博客系统，有以下几个实体： User （用户）、 Post （文章）、 Comment （评论）。
要求 ：
使用Gorm定义 User 、 Post 和 Comment 模型，其中 User 与 Post 是一对多关系（一个用户可以发布多篇文章）， Post 与 Comment 也是一对多关系（一篇文章可以有多个评论）。
编写Go代码，使用Gorm创建这些模型对应的数据库表。
题目2：关联查询
基于上述博客系统的模型定义。
要求 ：
编写Go代码，使用Gorm查询某个用户发布的所有文章及其对应的评论信息。
编写Go代码，使用Gorm查询评论数量最多的文章信息。
题目3：钩子函数
继续使用博客系统的模型。
要求 ：
为 Post 模型添加一个钩子函数，在文章创建时自动更新用户的文章数量统计字段。
为 Comment 模型添加一个钩子函数，在评论删除时检查文章的评论数量，如果评论数量为 0，则更新文章的评论状态为 "无评论"。
*/

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
	// 自动迁移模型
	//err := DB.AutoMigrate(&User{}, &Post{}, &Comment{})
	//if err != nil {
	//	log.Fatalf("数据库迁移失败: %v", err)
	//}
	//
	//fmt.Println("数据库表创建成功!")
	//// 创建用户
	user := User{
		Username: "lisa",
		Email:    "lisa@example.com",
		Password: "123456",
	}
	DB.Create(&user)
	//
	//// 创建文章
	post := Post{
		Title:   "GORM钩子函数教程1",
		Content: "这是一篇关于GORM钩子函数的文章...",
		UserID:  user.ID,
	}
	DB.Create(&post)
	//
	//// 创建评论
	comment := Comment{
		Content: "非常实用的教程",
		UserID:  user.ID,
		PostID:  post.ID,
	}
	DB.Create(&comment)

	// 删除评论(会自动触发AfterDelete钩子)
	DB.Delete(&comment)
	// 此时会检查并更新post的CommentStatus
}

// User 用户模型
type User struct {
	gorm.Model
	Username   string `gorm:"size:50;not null;unique"`
	Email      string `gorm:"size:100;not null;unique"`
	Password   string `gorm:"size:100;not null"`
	PostsCount int    `gorm:"default:0"` // 新增用户文章计数字段
	Posts      []Post // 一对多关系: 一个用户有多篇文章
}

// Post 文章模型
type Post struct {
	gorm.Model
	Title         string `gorm:"size:200;not null"`
	Content       string `gorm:"type:text;not null"`
	CommentCount  int    `gorm:"default:0"`             // 文章评论计数
	CommentStatus string `gorm:"size:20;default:'无评论'"` // 评论状态
	UserID        uint
	User          User
	Comments      []Comment
	CommentsCount int `gorm:"-"` // 不映射到数据库的字段
}

// Comment 评论模型
type Comment struct {
	gorm.Model
	Content string `gorm:"type:text;not null"`
	UserID  uint   // 评论者ID
	PostID  uint   // 所属文章ID
	Post    Post   // 属于Post的关系
	User    User   // 属于User的关系
}

func (p *Post) AfterCreate(tx *gorm.DB) (err error) {
	// 更新用户的PostsCount
	result := tx.Model(&User{}).Where("id = ?", p.UserID).
		Update("posts_count", gorm.Expr("posts_count + ?", 1))

	if result.Error != nil {
		return result.Error
	}

	// 如果更新影响行数为0，说明用户不存在
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (c *Comment) AfterDelete(tx *gorm.DB) (err error) {
	// 首先获取该文章的剩余评论数量
	var count int64
	if err := tx.Model(&Comment{}).
		Where("post_id = ?", c.PostID).
		Count(&count).Error; err != nil {
		return err
	}

	// 更新文章的评论状态
	updateData := map[string]interface{}{
		"comment_count": count,
	}

	if count == 0 {
		updateData["comment_status"] = "无评论"
	} else {
		updateData["comment_status"] = "有评论"
	}

	// 执行更新
	if err := tx.Model(&Post{}).
		Where("id = ?", c.PostID).
		Updates(updateData).Error; err != nil {
		return err
	}

	return nil
}

// 查询指定用户的所有文章及其评论
func getUserPostsWithComments(userID uint) ([]Post, error) {
	var posts []Post

	// 使用Preload预加载关联数据
	err := DB.Where("user_id = ?", userID).
		Preload("Comments").
		Preload("Comments.User"). // 同时加载评论的作者信息
		Find(&posts).Error

	/*
		SELECT * FROM `users` WHERE `users`.`id` = 1 AND `users`.`deleted_at` IS NULL
		SELECT * FROM `comments` WHERE `comments`.`post_id` = 1 AND `comments`.`deleted_at` IS NULL
		SELECT * FROM `posts` WHERE user_id = 1 AND `posts`.`deleted_at` IS NULL
	*/

	if err != nil {
		return nil, fmt.Errorf("查询用户文章失败: %v", err)
	}

	return posts, nil
}

// 使用示例
func example1() {
	posts, err := getUserPostsWithComments(1) // 查询用户ID为1的所有文章
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Printf("\n用户文章及评论:\n")
	for _, post := range posts {
		fmt.Printf("文章ID: %d, 标题: %s\n", post.ID, post.Title)
		fmt.Printf("评论数: %d\n", len(post.Comments))
		for _, comment := range post.Comments {
			fmt.Printf("  - 评论ID: %d, 内容: %s, 评论者: %s\n",
				comment.ID, comment.Content, comment.User.Username)
		}
		fmt.Println("------")
	}
}

// 查询评论数量最多的文章
func getMostCommentedPost() (*Post, error) {
	var post Post

	// 使用子查询和JOIN
	err := DB.
		Select("posts.*").
		Joins("LEFT JOIN comments ON comments.post_id = posts.id").
		Group("posts.id").
		Order("COUNT(comments.id) DESC").
		Preload("Comments").
		Preload("User").
		Preload("Comments.User").
		First(&post).Error
	/*
		[0.455ms] [rows:1] SELECT * FROM `comments` WHERE `comments`.`post_id` = 1 AND `comments`.`deleted_at` IS NULL
		[0.488ms] [rows:1] SELECT * FROM `users` WHERE `users`.`id` = 1 AND `users`.`deleted_at` IS NULL
		[0.364ms] [rows:1] SELECT * FROM `users` WHERE `users`.`id` = 1 AND `users`.`deleted_at` IS NULL
		[2.350ms] [rows:1] SELECT posts.* FROM `posts` LEFT JOIN comments ON comments.post_id = posts.id WHERE `posts`.`deleted_at` IS NULL GROUP BY `posts`.`id` ORDER BY COUNT(comments.id) DESC,`posts`.`id` LIMIT 1
		[0.322ms] [rows:1] SELECT count(*) FROM `comments` WHERE post_id = 1 AND `comments`.`deleted_at` IS NULL
	*/
	if err != nil {
		return nil, fmt.Errorf("查询最多评论文章失败: %v", err)
	}

	// 获取评论总数
	var commentCount int64
	DB.Model(&Comment{}).Where("post_id = ?", post.ID).Count(&commentCount)
	post.CommentsCount = int(commentCount) // 假设Post结构体添加了CommentsCount字段

	return &post, nil
}

// 使用示例
func example2() {
	post, err := getMostCommentedPost()
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Printf("\n评论最多的文章:\n")
	fmt.Printf("文章ID: %d, 标题: %s\n", post.ID, post.Title)
	fmt.Printf("作者: %s, 评论数: %d\n", post.User.Username, post.CommentsCount)
	for _, comment := range post.Comments {
		fmt.Printf("  - 评论: %s (by %s)\n", comment.Content, comment.User.Username)
	}
}
