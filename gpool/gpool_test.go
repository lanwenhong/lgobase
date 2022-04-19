package gpool

import (
    "testing"
    "time"
    "context"
    "github.com/lanwenhong/lgobase/gpool/gen-go/echo"
    "github.com/lanwenhong/lgobase/gpool"
)


func TestClientClient(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(),10 * time.Second)
    defer cancel()

    gp := &gpool.Gpool[echo.EchoClient]{}
    gp.GpoolInit("127.0.0.1", 9898, 3, 10, 5, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

    gc, _ := gp.Get()
    defer gc.Close()
    req := &echo.EchoReq{Msg:"You are welcome."}
    client := gc.Gc.GetThrfitClient()
    _, err := client.Echo(ctx, req)
    if err != nil {
        t.Fatal(err.Error())
    }

}
