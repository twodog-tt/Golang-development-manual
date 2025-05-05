package main

// https://leetcode.cn/problems/my-calendar-i/description/
func main() {

}

type pair struct{ start, end int }
type MyCalendar []pair

func Constructor() MyCalendar {
	return MyCalendar{}
}

func (this *MyCalendar) Book(startTime int, endTime int) bool {
	for _, p := range *this {
		if endTime > p.start && p.end > startTime {
			return false
		}
	}
	*this = append(*this, pair{startTime, endTime})
	return true
}
