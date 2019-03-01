package main

import (
	"bufio"
	"camel/examples/demo"
	"camel/lizard"
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/leesper/holmes"
)

type Demo struct {
}

func main() {
	defer holmes.Start().Stop()
	method := make(map[string]lizard.Serial)
	method["DemoOp2"] = lizard.Serial{
		Req:  1001,
		Resp: 1002,
	}
	lizard.RegisterService(100, &Demo{})
	lizard.InitServiceInvokeMap(method)
	// lizard.Register(1000001, &demo.DemoReq{}, DemoOp)
	// lizard.RegisterService2("100", DemoOp3)

	c, err := net.Dial("tcp", "127.0.0.1:10000")
	if err != nil {
		holmes.Fatalln(err)
	}

	conn := lizard.NewClientConn(0, c)
	defer conn.Close()

	conn.Start()
	for {
		reader := bufio.NewReader(os.Stdin)
		talk, _ := reader.ReadString('\n')
		if string(talk) == "q" {
			break
		} else {
			req := &demo.DemoReq{
				Call: strings.TrimSuffix(talk, "\n"),
			}
			fmt.Println("===>>req:", req.String())
			data, _ := proto.Marshal(req)
			Send(conn, data)
			// msg := chat.Message{
			// 	Content: talk,
			// }
			// if err := conn.Write(msg); err != nil {
			// 	holmes.Infoln("error", err)
			// }
		}
	}
	fmt.Println("goodbye")

}

func DecodeDemoResp(data []byte) (interface{}, error) {
	req := &demo.DemoReq{}
	err := proto.Unmarshal(data, req)
	return req, err
}

func DemoOp(ctx context.Context, msg interface{}) (interface{}, error) {
	req := msg.(*demo.DemoReq)
	fmt.Printf("====>>demo client op:%+v\n", req)
	resp := &demo.DemoResp{
		Reply: "oK, reply!",
	}
	return resp, nil
}

func (d *Demo) DemoOp2(ctx context.Context, msg *demo.DemoReq) *demo.DemoResp {
	fmt.Printf("====>>server demo op2:%+v\n", msg)
	resp := &demo.DemoResp{
		Reply: "oK, reply!",
	}
	return resp
}

func DemoOp3(ctx context.Context, msg *demo.DemoReq) *demo.DemoResp {
	fmt.Printf("====>>server demo op2:%+v\n", msg)
	resp := &demo.DemoResp{
		Reply: "oK, reply!",
	}
	return resp
}

func Send(conn *lizard.ClientConn, data []byte) {
	msg := &lizard.ZMessage{
		Source: int16(lizard.Source_User),
		ReqID:  1001,
		Data:   data,
	}
	conn.Write(*msg)
}
