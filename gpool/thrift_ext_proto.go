package gpool

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

const (
	THRIFT_EXT_META_MAGIC   = int16(0x7FFF)
	THRIFT_EXT_META_VERSION = int16(1)
)

// type RequestIDProtocolClient struct {
type ThriftExtProtocolClient struct {
	thrift.TProtocol
	baseFactory thrift.TProtocolFactory
}

type ExtProcessor struct {
	thrift.TProcessor
	Processor thrift.TProcessor
	Pro       *thrift.TBinaryProtocolFactory
}

type ExtContext struct {
	context.Context
	ReqExtData map[string]string
	RspExtData map[string]string
}

func GenerateRequestID() string {
	return uuid.New().String()
}

func GetRequestID(ctx context.Context) string {
	if rid, ok := ctx.Value("request_id").(string); ok {
		return rid
	}
	return ""
}

func WithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, "request_id", rid)
}

func NewExtContext(ctx context.Context) *ExtContext {
	nCtx := &ExtContext{
		Context: ctx,
	}
	nCtx.ReqExtData = make(map[string]string)
	return nCtx
}

func (ec *ExtContext) SetReqExtData(ctx context.Context, k, v string) *ExtContext {
	//newValues := make(map[string]string)
	/*for k, v := range ec.ReqExtData {
		/*if k != "trace_id" {
			newValues[k] = v
		}
		logger.Debugf(ctx, "k: %s v: %s", k, v)
		ctx = context.WithValue(ctx, k, v)
	}*/
	//logger.Debugf(ctx, "k: %s v: %s", k, v)
	//logger.Debugf(ctx, "newValues: %v", newValues)
	//newValues[k] = v
	ec.ReqExtData[k] = v
	ctx = context.WithValue(ctx, k, v)
	//id := ctx.Value("trace_id").(string)
	//logger.Debugf(ctx, id)
	return &ExtContext{
		Context: ctx,
		//ReqExtData: newValues,
		ReqExtData: ec.ReqExtData,
	}
}

func (ec *ExtContext) GetReqExtData(k string) string {
	if v, ok := ec.ReqExtData[k]; ok {
		return v
	}
	return ""
}

/*func NewRequestIDProtocolClient(baseFactory thrift.TProtocolFactory) *RequestIDProtocolClient {
	return &RequestIDProtocolClient{
		baseFactory: baseFactory,
	}
}

func NewRequestIDProtocolNewClient(proto thrift.TProtocol) *RequestIDProtocolClient {
	return &RequestIDProtocolClient{
		TProtocol: proto,
	}
}

func (p *RequestIDProtocolClient) GetProtocol(transport thrift.TTransport) thrift.TProtocol {
	baseProto := p.baseFactory.GetProtocol(transport)
	return NewRequestIDProtocolNewClient(baseProto)
}*/

func NewThriftExtProtocolClient(baseFactory thrift.TProtocolFactory) *ThriftExtProtocolClient {
	return &ThriftExtProtocolClient{
		baseFactory: baseFactory,
	}
}

func NewThriftExtProtocolNewClient(proto thrift.TProtocol) *ThriftExtProtocolClient {
	return &ThriftExtProtocolClient{
		TProtocol: proto,
	}
}

func (p *ThriftExtProtocolClient) GetProtocol(transport thrift.TTransport) thrift.TProtocol {
	baseProto := p.baseFactory.GetProtocol(transport)
	return NewThriftExtProtocolNewClient(baseProto)
}

func (p *ThriftExtProtocolClient) WriteMessageBegin(ctx context.Context, name string, typeId thrift.TMessageType, seqId int32) error {
	//starttime := time.Now()
	//magic
	err := p.WriteI16(ctx, THRIFT_EXT_META_MAGIC)
	if err != nil {
		logger.Warnf(ctx, "write magic err: %v", err)
		return err
	}
	//ver
	err = p.WriteI16(ctx, THRIFT_EXT_META_VERSION)
	if err != nil {
		logger.Warnf(ctx, "write version err: %v", err)
		return err
	}
	//ext data
	nCtx := ctx.(*ExtContext)
	err = p.WriteMapBegin(ctx, thrift.STRING, thrift.STRING, len(nCtx.ReqExtData))
	if err != nil {
		logger.Warnf(ctx, "write map begin err: %", err)
		return err
	}
	for k, v := range nCtx.ReqExtData {
		if err = p.WriteString(ctx, k); err != nil {
			logger.Warnf(ctx, "write map k: %s err: %v", k, err)
			return err
		}
		if err = p.WriteString(ctx, v); err != nil {
			logger.Warnf(ctx, "write map v: %s err: %v", v, err)
			return err
		}
	}
	p.WriteMapEnd(ctx)
	//logger.Infof(ctx, "func=WriteMessageBegin|time=%v", time.Since(starttime))
	return p.TProtocol.WriteMessageBegin(ctx, name, typeId, seqId)
}

func NewExtProcessor(processor thrift.TProcessor, p *thrift.TBinaryProtocolFactory) *ExtProcessor {
	return &ExtProcessor{
		Processor: processor,
		Pro:       p,
	}
}

func (p *ExtProcessor) ReadMagic(ctx context.Context, in, out thrift.TProtocol) (int16, []byte, bool, thrift.TException) {
	//buf := make([]byte, 2)
	buf := new(bytes.Buffer)
	magic, err := in.ReadI16(ctx)
	if err != nil {
		if err.(thrift.TProtocolException).TypeId() == thrift.END_OF_FILE {
			return magic, buf.Bytes(), false, thrift.NewTTransportException(thrift.END_OF_FILE, "connection closed (EOF)")
		} else {
			return magic, buf.Bytes(), false, thrift.NewTTransportException(thrift.INVALID_DATA, "invalid data")
		}
	}
	binary.Write(buf, binary.BigEndian, uint16(magic))

	return magic, buf.Bytes(), true, nil
}

func (p *ExtProcessor) ReadMetaVer(ctx context.Context, in, out thrift.TProtocol) (int16, bool, thrift.TException) {
	ver, err := in.ReadI16(ctx)
	if err != nil {
		if err.(thrift.TProtocolException).TypeId() == thrift.END_OF_FILE {
			return ver, false, thrift.NewTTransportException(thrift.END_OF_FILE, "connection closed (EOF)")
		} else {
			return ver, false, thrift.NewTTransportException(thrift.INVALID_DATA, "invalid data")
		}
	}
	logger.Debugf(ctx, "ver: %d", ver)
	return ver, true, nil
}

// func (p *ExtProcessor) ReadMetaMap(ctx context.Context, in, out thrift.TProtocol) (map[string]string, error) {
func (p *ExtProcessor) ReadMetaMap(ctx context.Context, in, out thrift.TProtocol) (context.Context, error) {
	//reqData := make(map[string]string)
	_, _, size, err := in.ReadMapBegin(ctx)
	if err != nil {
		logger.Warnf(ctx, "read map header err: %v", err)
		//return ctx, reqData, err
		return ctx, err
	}
	logger.Debugf(ctx, "size: %d", size)
	req_id := ""
	for i := 0; i < size; i++ {
		k, err := in.ReadString(ctx)
		if err != nil {
			logger.Warnf(ctx, "read map key err: %v", err)
			//return ctx, reqData, err
			return ctx, err
		}
		v, err := in.ReadString(ctx)
		if err != nil {
			logger.Warnf(ctx, "read map v err: %v", err)
			//return ctx, reqData, err
			return ctx, err
		}
		logger.Debugf(ctx, "k=%s v=%s", k, v)
		//reqData[k] = v
		ctx = context.WithValue(ctx, k, v)

		if k == "request_id" {
			req_id = v
		}
	}
	if req_id == "" {
		ctx = context.WithValue(ctx, "request_id", util.NewRequestID())
	}
	//in.ReadMapEnd(ctx)
	//return ctx, reqData, err
	return ctx, err
}

func (p *ExtProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	preBuf := make([]byte, 2)
	//starttime := time.Now()
	_, err := io.ReadFull(in.Transport(), preBuf)
	//magic, preBuf, _, err := p.ReadMagic(ctx, in, out)
	//logger.Infof(ctx, "func=Process|time=%v", time.Since(starttime))
	if err != nil {
		if err.(thrift.TProtocolException).TypeId() == thrift.END_OF_FILE {
			return false, thrift.NewTTransportException(thrift.END_OF_FILE, "connection closed (EOF)")
		} else {
			logger.Warnf(ctx, "read preBuf: %v", err)
			return false, thrift.NewTTransportException(thrift.INVALID_DATA, "invalid data")
		}
	}
	//magic
	magic := binary.BigEndian.Uint16(preBuf)
	if magic == uint16(THRIFT_EXT_META_MAGIC) {
		//if magic == THRIFT_EXT_META_MAGIC {
		logger.Debugf(ctx, "use extend thrift proto")
		//ver
		_, flag, ex := p.ReadMetaVer(ctx, in, out)
		if ex != nil {
			return flag, ex
		}
		//map
		//reqData, err := p.ReadMetaMap(ctx, in, out)
		var errRM error
		if ctx, errRM = p.ReadMetaMap(ctx, in, out); errRM != nil {
			if errRM.(thrift.TProtocolException).TypeId() == thrift.END_OF_FILE {
				return false, thrift.NewTTransportException(thrift.END_OF_FILE, "connection closed (EOF)")
			} else {
				logger.Warnf(ctx, "read preBuf: %v", errRM)
				return false, thrift.NewTTransportException(thrift.INVALID_DATA, "invalid data")
			}
		}
		/*for k, v := range reqData {
			ctx = context.WithValue(ctx, k, v)
		}*/

	} else {
		logger.Debugf(ctx, "use normal thrift proto")
		multiReader := io.MultiReader(bytes.NewReader(preBuf), in.Transport())
		newTransport := thrift.NewStreamTransportR(multiReader)
		in = p.Pro.GetProtocol(newTransport)
		ctx = context.WithValue(ctx, "request_id", util.NewRequestID())
	}
	//b, t := p.Processor.Process(newCtx, in, out)
	b, t := p.Processor.Process(ctx, in, out)
	return b, t
}
