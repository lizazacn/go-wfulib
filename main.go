package main

import (
	"compress/flate"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/axgle/mahonia"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	URL "net/url"
	"regexp"
	"strings"
	"time"
)

type OrderResult struct {
	Msg string `json:"msg"`
	Status bool `json:"success"`
}

type OrderTimeSlot struct {
	StartTime string
	EndTime string
}

type Library struct {
	header http.Header
	userInfo *UserInfo
}

type UserInfo struct {
	User string
	Passwd string
	SeatId string
	RoomId string
	days int
	StartHour int
	StartMinute int
}

func request(url, data string, header http.Header, method string, cookies ...*http.Cookie) *http.Response {
	//uri, _ := URL.Parse("http://127.0.0.1:8866")
	//client := &http.Client{
	//	Transport: &http.Transport{
	//		Proxy: http.ProxyURL(uri),
	//	},
	//}
	client := http.Client{}
	req, err := http.NewRequest(method, url, strings.NewReader(data))
	if err != nil {
		log.Printf("创建Request请求异常：%v\n", err)
		return nil
	}
	req.Header = header
	if len(cookies) > 0{
		for i := range cookies{
			req.AddCookie(cookies[i])
		}
	}
	response, err := client.Do(req)
	if err != nil{
		log.Printf("拉取用户信息异常：%v\n", err)
		return nil
	}
	return response
}

// ConvertToString 字符编码转换函数
func ConvertToString(src string, srcCode string, tagCode string) string {

	srcCoder := mahonia.NewDecoder(srcCode)

	srcResult := srcCoder.ConvertString(src)

	tagCoder := mahonia.NewDecoder(tagCode)

	_, cdata, _ := tagCoder.Translate([]byte(srcResult), true)

	result := string(cdata)

	return result

}

// 对压缩的html页面信息进行处理
func switchContentEncoding(res *http.Response) (bodyReader io.Reader, err error) {
	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		bodyReader, err = gzip.NewReader(res.Body)
	case "deflate":
		bodyReader = flate.NewReader(res.Body)
	default:
		bodyReader = res.Body
	}
	return
}

func LibraryInit() *Library {
	var library = new(Library)
	library.header = http.Header{}
	library.header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	library.header.Add("Accept-Encoding", "gzip, deflate")
	library.header.Add("Accept-Language", "zh-CN,zh;q=0.9")
	library.header.Add("Cache-Control", "no-cache")
	library.header.Add("Connection", "keep-alive")
	library.header.Add("Pragma", "no-cache")
	library.header.Add("Upgrade-Insecure-Requests", "1")
	library.header.Add("User-Agent", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Mobile Safari/537.36")
	library.header.Add("Content-Type", "application/x-www-form-urlencoded")
	return library
}

func (lib *Library)login() []*http.Cookie{
	reLogin:
	login_url := "http://passport2.chaoxing.com/fanyalogin"
	login_data := map[string]string{
		"uname": lib.userInfo.User,
		"password": base64.StdEncoding.EncodeToString([]byte(lib.userInfo.Passwd)),
		"refer": "http%3A%2F%2Foffice.chaoxing.com%2Ffront%2Fthird%2Fapps%2Fseat%2Findex%3FfidEnc%3D35bbd135397006a8",
		"t": "true",
		"fid": "-1",
	}

	DataUrlVal := URL.Values{}
	for key,val := range login_data{
		DataUrlVal.Add(key,val)
	}

	response := request(login_url, DataUrlVal.Encode(), lib.header, http.MethodPost)
	if response == nil{
		goto reLogin
	}
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusSwitchingProtocols {
		respStr, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Printf("读取返回数据异常：%v\n", err)
		}
		fmt.Println(string(respStr))
	}
	return response.Cookies()
}

func (lib *Library)getPageToken() (string, []*http.Cookie) {
	cookies := lib.login()
	url := "http://office.chaoxing.com/front/third/apps/seat/index?fidEnc=35bbd135397006a8"
	reLoad:
	response := request(url, "", lib.header, http.MethodGet, cookies...)
	if response == nil{
		goto reLoad
	}
	body, err := switchContentEncoding(response)
	if err != nil{
		log.Printf("解压页面内容异常：%v", err)
	}
	newCookies := response.Cookies()
	responseText, err := io.ReadAll(body)
	if err != nil{
		log.Printf("读取响应数据异常：%v", err)
	}

	if len(responseText) > 0{
		tokenRegexp := regexp.MustCompile("'&pageToken=' \\+ '(\\w*)'")
		token := tokenRegexp.Find(responseText)
		fmt.Println(string(token))
		return string(token[17:][:len(token)-18]),newCookies
	}
	return "", newCookies
}

func (lib *Library)getToken(roomId, day string) string{
	pageToken, cookies := lib.getPageToken()
	if len(pageToken) <= 0{
		return ""
	}
	lib.header.Del("Cookie")
	url := fmt.Sprintf("http://office.chaoxing.com/front/third/apps/seat/select?id=%s&day=%s&backLevel=2&pageToken=%s&fidEnc=35bbd135397006a8", roomId, day, pageToken)
	reLoad:
	response := request(url, "", lib.header, http.MethodGet, cookies...)
	if response == nil{
		goto reLoad
	}
	body, err := switchContentEncoding(response)
	if err != nil{
		log.Printf("解压页面内容异常：%v", err)
	}
	responseText, err := io.ReadAll(body)
	if err != nil{
		log.Printf("读取响应数据异常：%v", err)
	}

	if len(responseText) > 0{
		tokenRegexp := regexp.MustCompile("token: '(\\w*)'")
		token := tokenRegexp.Find(responseText)
		fmt.Println(string(token))
		if len(token) > 0{
			return string(token[8:][:len(token)-9])
		}
		return ""
	}
	return ""
}

func (lib *Library)OrderSeat(roomId, startTime, endTime, day, seatId, token string)  {
	var orderResult OrderResult
	var count int8
reLoad:
	orderUrl := fmt.Sprintf("http://office.chaoxing.com/data/apps/seat/submit?roomId=%s&startTime=%s&endTime=%s&day=%s&seatNum=%s&captcha=&token=%s", roomId, startTime, endTime, day, seatId, token)
	response := request(orderUrl, "", lib.header, http.MethodGet)
	if response == nil{
		goto reLoad
	}
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusSwitchingProtocols {
		respStr, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Printf("读取返回数据异常：%v\n", err)
		}
		err = json.Unmarshal(respStr, &orderResult)
		if err != nil {
			log.Printf("解析返回数据异常：%v\n", err)
		}
		log.Printf(string(respStr))
		if orderResult.Status{
			return
		}
		log.Println(orderResult.Msg)
		if strings.Contains(orderResult.Msg, "被占用") {
			return
		}
		if strings.Contains(orderResult.Msg, "上限"){
			return
		}
		if strings.Contains(orderResult.Msg, "非法"){
			count += 1
			if count < 2{
				goto reLoad
			}
			return
		}
	}
	return
}



func OrderTodaySeat(info *UserInfo, day string) {
	var timeSlotArray [7]*OrderTimeSlot = [7]*OrderTimeSlot{
		&OrderTimeSlot{
			StartTime: "8:00",
			EndTime: "10:00",
		},
		&OrderTimeSlot{
			StartTime: "10:00",
			EndTime: "12:00",
		},
		&OrderTimeSlot{
			StartTime: "12:00",
			EndTime: "14:00",
		},
		&OrderTimeSlot{
			StartTime: "14:00",
			EndTime: "16:00",
		},
		&OrderTimeSlot{
			StartTime: "16:00",
			EndTime: "18:00",
		},
		&OrderTimeSlot{
			StartTime: "18:00",
			EndTime: "20:00",
		},
		&OrderTimeSlot{
			StartTime: "20:00",
			EndTime: "21:00",
		},
	}
	//quitChan := make(chan bool, 7)
	for i := range timeSlotArray{
		lib := LibraryInit()
		lib.userInfo = info
		token := lib.getToken(info.RoomId, day)
		lib.OrderSeat(info.RoomId, timeSlotArray[i].StartTime, timeSlotArray[i].EndTime, day, info.SeatId, token)
	}
}

func run(info *UserInfo) {
	Run := make(chan bool, 2)

	go func(ch chan bool) {
		fmt.Println("预约定时器启动！")
		ch <- true
		for true{
			time.Sleep(500 * time.Millisecond)
			if time.Now().Hour() == info.StartHour && time.Now().Minute() == info.StartMinute{
				ch <- true
				time.Sleep(2 * time.Minute)
			}
		}
	}(Run)
	Next:
	<- Run
	roomId := info.RoomId
	//roomId := "6258"
	day := time.Now().AddDate(0,0,info.days).Format("2006-01-02")
	seatId := info.SeatId
	log.Printf("获取日期，座位ID等参数成功！》》》》》》》》》》》ROOM:%s,seatId:%s", roomId,seatId)
	start := time.Now().Unix()
	OrderTodaySeat(info, day)
	end := time.Now().Unix()
	fmt.Printf("预约全部时段攻击耗时%d秒!\n",end-start)
	goto Next
}

func main() {
	userInfo := new(UserInfo)
	flag.StringVar(&userInfo.User,"uname", "", "用户名")
	flag.StringVar(&userInfo.Passwd,"passwd", "", "密码")
	flag.StringVar(&userInfo.RoomId,"room", "", "房间号")
	flag.StringVar(&userInfo.SeatId,"seat", "", "座位号")
	flag.IntVar(&userInfo.days,"days", 1, "预约时间:0表示当天，1表示第二天以此类推")
	flag.IntVar(&userInfo.StartHour,"hour", 21, "开始预约时间:*时")
	flag.IntVar(&userInfo.StartMinute,"minute", 30, "开始预约时间:*分")
	flag.Parse()
	fmt.Printf("程序启动时间：%d:%d\n", userInfo.StartHour, userInfo.StartMinute)
	run(userInfo)
}