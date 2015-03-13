package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"net"
	"time"
)

var (
	online = make(map[net.Conn]string)
)

type JSON struct {
	TYPE    string
	CONTENT []string
}

func main() {
	listen, err := net.Listen("tcp", ":3333")
	if err != nil {
		fmt.Println("监听端口失败:", err.Error())
		return
	}
	fmt.Println("已初始化连接，等待客户端连接...")
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("Accept Error:", err.Error())
			return
		}
		fmt.Println("客户端连接来自:", conn.RemoteAddr().String())
		defer conn.Close()
		go Server(conn)
	}
}

func Server(conn net.Conn) {
	for {
		var sour JSON
		sour, err := rcvMsg(conn)
		if err != nil {
			fmt.Println("读取客户端数据错误:", err.Error())
			delete(online, conn)
			break
		}
		fmt.Println(sour)
		switch sour.TYPE {
		case "LOGIN":
			loginHandler(conn, sour)
		case "MSG":
			msgHandler(conn, sour)
		case "ADDFRIEND":
			addfriendHandler(conn, sour)
		case "DELFRIEND":
			delfriendHandler(conn, sour)
		case "ADDGROUP":
			addgroupHandler(conn, sour)
		case "DELGROUP":
			delgroupHandler(conn, sour)
		case "MOVFRIEND":
			movefriendHandler(conn, sour)
		default:
		}
	}
}
func msgHandler(conn net.Conn, sour JSON) {
	var send JSON
	t := time.Now().Format("01-02\t15:04:05")
	to := sour.CONTENT[1]
	send.TYPE = "MSG"
	send.CONTENT = append(send.CONTENT, sour.CONTENT[0], sour.CONTENT[1], t, sour.CONTENT[3])
	for i := range online {
		if to == online[i] {
			sendMsg(i, send)
		}
		fmt.Println(send)
		//sendMsg(i, send)
	}
	sendMsg(conn, send)
}

func sendMsg(conn net.Conn, send JSON) {
	END := []byte("\r\n")
	str, _ := json.Marshal(send)
	str = append(str, END...)
	conn.Write(str)
}

func rcvMsg(conn net.Conn) (JSON, error) {
	var rcv JSON
	data := make([]byte, 1024)
	i, err := conn.Read(data)
	fmt.Println("客户端", conn.RemoteAddr().String(), "发来数据:", string(data[0:i]))
	json.Unmarshal(data[0:i], &rcv)
	return rcv, err
}

func loginHandler(conn net.Conn, sour JSON) {
	var send JSON
	userName := sour.CONTENT[0]
	passWord := sour.CONTENT[1]

	db, _ := sql.Open("mysql", "root:root@/android?charset=utf8")
	rows, _ := db.Query("select uid,password from users where username=?", userName)
	uid := 0
	pw := ""
	for rows.Next() {
		rows.Scan(&uid, &pw)
	}
	send.TYPE = "LOGIN"
	if passWord == pw && userName != "" {
		online[conn] = userName
		send.CONTENT = append(send.CONTENT, "登陆成功")
		//friendListHandler(conn)
	} else {
		if uid > 0 {
			send.CONTENT = append(send.CONTENT, "FAIL", "没有此用户")
		} else {
			send.CONTENT = append(send.CONTENT, "FAIL", "密码错误")
		}
	}
	sendMsg(conn, send)
}

func friendListHandler(conn net.Conn) {
	//fmt.Println(online[conn] + "FLHandler")
	db, _ := sql.Open("mysql", "root:root@/android?charset=utf8")
	rows, _ := db.Query("select mgroup from groups where user=?", online[conn])
	var send JSON
	send.TYPE = "FRIENDGROUP"
	for rows.Next() {
		send.CONTENT = append(send.CONTENT, "")
		mgroup := ""
		rows.Scan(&mgroup)
		send.CONTENT = append(send.CONTENT, mgroup)
		rows2, _ := db.Query("select friend from friends where user=? and groups=?", online[conn], mgroup)
		for rows2.Next() {
			friend := ""
			rows2.Scan(&friend)
			send.CONTENT = append(send.CONTENT, friend)
		}
	}
	fmt.Println(send)
	sendMsg(conn, send)
}

func addfriendHandler(conn net.Conn, sour JSON) {
	var send JSON
	send.TYPE = "ADDFRIEND"
	user := online[conn]
	friend := sour.CONTENT[0]
	group := sour.CONTENT[1]
	db, _ := sql.Open("mysql", "root:root@/android?charset=utf8")
	rows, _ := db.Query("select uid from users where username=?", friend)
	uid := 0
	for rows.Next() {
		rows.Scan(&uid)
	}
	if uid > 0 {
		rows, _ = db.Query("select uid from friends where user=? and friend=?", user, friend)
		uid = 0
		for rows.Next() {
			rows.Scan(&uid)
		}
		if uid > 0 {
			send.CONTENT = append(send.CONTENT, "要添加的好友已在你的好友列表")
		} else {
			if user == friend {
				send.CONTENT = append(send.CONTENT, "不能添加自己")
				friendListHandler(conn)
			} else {
				db.Exec("insert into friends(user,friend,groups) value(?,?,?)", user, friend, group)
				send.CONTENT = append(send.CONTENT, "添加成功")
				friendListHandler(conn)
			}
		}
	} else {
		send.CONTENT = append(send.CONTENT, "要添加的好友不存在")
	}
	sendMsg(conn, send)
}

func delfriendHandler(conn net.Conn, sour JSON) {
	var send JSON
	send.TYPE = "DELFRIEND"
	user := online[conn]
	friend := sour.CONTENT[0]
	db, _ := sql.Open("mysql", "root:root@/android?charset=utf8")
	_, err := db.Exec("delete from friends where user=? and friend=?", user, friend)
	if err != nil {
		send.CONTENT = append(send.CONTENT, "删除失败")
	} else {
		send.CONTENT = append(send.CONTENT, "删除成功")
	}
	friendListHandler(conn)
	sendMsg(conn, send)
}

func movefriendHandler(conn net.Conn, sour JSON) {
	var send JSON
	send.TYPE = "MOVFRIEND"

	user := online[conn]
	who := sour.CONTENT[0]
	//	from := sour.CONTENT[1]
	to := sour.CONTENT[2]

	db, _ := sql.Open("mysql", "root:root@/android?charset=utf8")
	_, err := db.Exec("update friends set groups=? where user=? and friend=?", to, user, who)
	if err != nil {
		send.CONTENT = append(send.CONTENT, "转移失败")
	} else {
		send.CONTENT = append(send.CONTENT, "转移成功")
	}
	friendListHandler(conn)
	sendMsg(conn, send)
}

func addgroupHandler(conn net.Conn, sour JSON) {
	var send JSON
	send.TYPE = "ADDGROUP"
	user := online[conn]
	group := sour.CONTENT[0]

	db, _ := sql.Open("mysql", "root:root@/android?charset=utf8")
	rows, _ := db.Query("select uid from groups where user=? and mgroup=?", user, group)

	uid := 0
	for rows.Next() {
		rows.Scan(&uid)
	}

	if uid > 0 {
		send.CONTENT = append(send.CONTENT, "你已命名了该组")
	} else {
		_, err := db.Exec("insert into groups(user,mgroup) value(?,?)", user, group)
		if err != nil {
			send.CONTENT = append(send.CONTENT, "无法添加该组")
		} else {
			send.CONTENT = append(send.CONTENT, "添加成功")
		}
	}
	friendListHandler(conn)
	sendMsg(conn, send)
}

func delgroupHandler(conn net.Conn, sour JSON) {
	var send JSON
	send.TYPE = "DELGROUP"
	user := online[conn]
	inGroup := sour.CONTENT[0]
	delGroup := sour.CONTENT[1]

	db, _ := sql.Open("mysql", "root:root@/android?charset=utf8")
	db.Exec("delete from groups where user=? and mgroup=?", user, delGroup)
	_, err := db.Exec("update friends set groups=? where user=? and groups=?", inGroup, user, delGroup)
	if err != nil {
		send.CONTENT = append(send.CONTENT, "删除失败")
	} else {
		send.CONTENT = append(send.CONTENT, "删除成功")
	}
	friendListHandler(conn)
	sendMsg(conn, send)
}
