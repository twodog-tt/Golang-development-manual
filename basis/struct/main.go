package main

import "fmt"

func main() {
	//c := Circle{radius: 3}
	//fmt.Println(c.Area())
	//fmt.Println(c.Perimeter())
	//
	//r := Rectangle{width: 10, height: 5}
	//fmt.Println(r.Area())
	//fmt.Println(r.Perimeter())

	p := Person{Name: "td", Age: 20}
	e := Employee{EmployeeId: "007", person: p}
	e.PrintInfo()
}

// 题目 ：定义一个 Shape 接口，包含 Area() 和 Perimeter() 两个方法。然后创建 Rectangle 和 Circle 结构体，实现 Shape 接口。
// 在主函数中，创建这两个结构体的实例，并调用它们的 Area() 和 Perimeter() 方法。
// 考察点 ：接口的定义与实现、面向对象编程风格。

type Shape interface {
	Area(float64) float64
	Perimeter(float64) float64
}

func (c Circle) Area() float64 {
	return c.radius * c.radius * 3.14
}

func (r Rectangle) Area() float64 {
	return r.height * r.width
}

func (c Circle) Perimeter() float64 {
	return 2 * 3.14 * c.radius
}

func (r Rectangle) Perimeter() float64 {
	return 2 * (r.width + r.height)
}

type Circle struct {
	radius float64
}
type Rectangle struct {
	width, height float64
}

// 题目 ：使用组合的方式创建一个 Person 结构体，包含 Name 和 Age 字段，再创建一个 Employee 结构体，组合 Person 结构体并添加 EmployeeID 字段。
// 为 Employee 结构体实现一个 PrintInfo() 方法，输出员工的信息。
// 考察点 ：组合的使用、方法接收者。

type Person struct {
	Name string
	Age  int
}

type Employee struct {
	person     Person
	EmployeeId string
}

func (e Employee) PrintInfo() {
	fmt.Println("输出员工ID:", e.EmployeeId)
	fmt.Println("输出员工姓名:", e.person.Name)
	fmt.Println("输出员工年龄:", e.person.Age)
}
