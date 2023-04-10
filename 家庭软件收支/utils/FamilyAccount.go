package utils

import "fmt"

// 收支结构体
type familyAccount struct {
	//loop表示当前循环状态
	loop bool
	//保存用户输入的选项
	key string
	//账户余额
	balance float64
	//收支金额
	money float64
	//每次的收支说明
	notes string
	//记录是否有收支行为
	flag bool
	//每次收支行为过后的详情
	details string
}

// NewFamilyAccount 使用工厂模式的构造方法，返回一个*FamilyAccount的实例
func NewFamilyAccount() *familyAccount {
	return &familyAccount{
		key:     "",
		loop:    true,
		balance: 10000.0,
		money:   0.0,
		notes:   "",
		flag:    false,
		details: "收支\t 账户金额\t 收支金额\t 说明",
	}
}

// ShowDetails 显示收支明细
func (this *familyAccount) ShowDetails() {
	fmt.Println("----------------当前的收支明细记录是-----------------")
	if this.flag {
		fmt.Println(this.details)
	} else {
		fmt.Println("抱歉，目前还没有收支明细，快来添加一笔吧!")
	}
}

// InCome 登记收入
func (this *familyAccount) InCome() {
	fmt.Println("本次的收入金额是：")
	fmt.Scanln(&this.money)
	this.balance += this.money
	fmt.Println("本次收入说明:")
	fmt.Scanln(&this.notes)
	//拼接本次的收入情况
	this.details += fmt.Sprintf("\n收入\t %v\t\t %v\t\t %v", this.balance, this.money, this.notes)
	this.flag = true
}

// OutCome 登记支出
func (this *familyAccount) OutCome() {
	fmt.Println("本次的支出金额是:")
	fmt.Scanln(&this.money)
	if this.money > this.balance {
		fmt.Println("本次的余额不足，请注意收支平衡")
		//这里的默认情况是不退出此函数
	}
	this.balance -= this.money
	fmt.Println("本次的支出说明:")
	fmt.Scanln(&this.notes)
	//拼接本次的收入情况
	this.details += fmt.Sprintf("\n支出\t %v\t\t %v\t\t %v", this.balance, this.money, this.notes)
	this.flag = true
}

// Exit 退出主界面
func (this *familyAccount) Exit() {
	fmt.Println("选择退出与否Y/N")
	choice := ""
	for {
		fmt.Scanln(&choice)
		if choice == "y" || choice == "Y" {
			this.loop = false
			break
		} else if choice == "n" || choice == "N" {
			break
		} else {
			fmt.Println("输入有误，请重新输入")
		}
	}

}

// Menu 显示主界面
func (this *familyAccount) Menu() {
	for {
		fmt.Println("-------------------家庭收支记账 ----------------")
		fmt.Println("-------------------1.收支明细-------------------")
		fmt.Println("-------------------2.登记收入-------------------")
		fmt.Println("-------------------3.登记支出-------------------")
		fmt.Println("-------------------4.退出访问-------------------")
		fmt.Println("请输入（1-4）:")
		fmt.Scanln(&this.key)
		switch this.key {
		case "1":
			this.ShowDetails()
		case "2":
			this.InCome()
		case "3":
			this.OutCome()
		case "4":
			this.Exit()
		default:
			fmt.Println("请重新输入正确的选项")

		}
		//退出循环
		if !this.loop {
			break
		}
	}
}
