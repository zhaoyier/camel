package main

import (
	"context"
	"fmt"
	"net"

	"camel/examples/demo"
	"camel/lizard"

	"github.com/golang/protobuf/proto"
	"github.com/leesper/holmes"
)

func main() {
	defer holmes.Start().Stop()

	lizard.Register(1000000, &demo.DemoReq{}, DemoOp)

	c, err := net.Dial("tcp", "127.0.0.1:10000")
	if err != nil {
		holmes.Fatalln(err)
	}

	conn := lizard.NewClientConn(0, c)
	defer conn.Close()
	go RegisterServer(conn)
	conn.Start()
	// for {
	// 	reader := bufio.NewReader(os.Stdin)
	// 	talk, _ := reader.ReadString('\n')
	// 	if talk == "bye\n" {
	// 		break
	// 	} else {
	// 		msg := chat.Message{
	// 			Content: talk,
	// 		}
	// 		if err := conn.Write(msg); err != nil {
	// 			holmes.Infoln("error", err)
	// 		}
	// 	}
	// }
	for {

	}
	fmt.Println("goodbye")
}

func DecodeDemoReq(data []byte) (interface{}, error) {
	fmt.Printf("===>>server demo decode: %s\n", data)
	req := &demo.DemoReq{}
	err := proto.Unmarshal(data, req)
	return req, err
}

func DemoOp(ctx context.Context, msg interface{}) (interface{}, error) {
	req := msg.(*demo.DemoReq)
	fmt.Printf("====>>server demo op:%+v\n", req)
	resp := &demo.DemoResp{
		Reply: "oK, reply!",
	}
	return resp, nil
}

// 注册服务
func RegisterServer(conn *lizard.ClientConn) {
	msg := &lizard.ZMessage{
		Source: int16(lizard.Source_SRegister),
		ReqID:  2,
	}
	msg.Data, _ = proto.Marshal(&lizard.RegisterServerReq{
		Addr:   "127.0.0.1:10002",
		Modulo: 1,
	})
	msg.Length = int16(12 + len(msg.Data))
	conn.Write(*msg)
}
