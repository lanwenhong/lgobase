package gpool

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strconv"
	"strings"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

const (
	//THRIFT_EXT_META_MAGIC   = int16(0x7FFF)
	THRIFT_EXT_META_MAGIC          = int32(0x7FFFFFFF)
	THRIFT_EXT_META_VERSION        = int16(1)
	THRIFT_EXT_CALL_CLIENT_SERVICE = "call_client_service"
	THRIFT_EXT_CLIENT_SERVICE      = "client_service"
	THRIFT_EXT_DEPTH               = "trace_depth"
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

	/*if rid, ok := ctx.Value("request_id").(string); ok {
		nCtx.ReqExtData["request_id"] = rid
	}*/
	klist := strings.Split(logger.Gfilelog.Logconf.CtxValueKey, ",")
	for _, k := range klist {
		if v, ok := ctx.Value(k).(string); ok {
			nCtx.ReqExtData[k] = v
		}
	}
	return nCtx
}

func (ec *ExtContext) SetReqExtData(ctx context.Context, k, v string) *ExtContext {
	ec.ReqExtData[k] = v
	ctx = context.WithValue(ctx, k, v)
	return &ExtContext{
		Context:    ctx,
		ReqExtData: ec.ReqExtData,
	}
}

func (ec *ExtContext) SetReqExtCallClientService(ctx context.Context, v string) *ExtContext {
	ec.ReqExtData[THRIFT_EXT_CALL_CLIENT_SERVICE] = v
	ctx = context.WithValue(ctx, THRIFT_EXT_CALL_CLIENT_SERVICE, v)
	return &ExtContext{
		Context:    ctx,
		ReqExtData: ec.ReqExtData,
	}
}

func (ec *ExtContext) GetReqExtData(k string) string {
	if v, ok := ec.ReqExtData[k]; ok {
		return v
	}
	if m := ec.Value(k); m != nil {
		return m.(string)
	}
	return ""
}

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
	//err := p.WriteI16(ctx, THRIFT_EXT_META_MAGIC)
	err := p.WriteI32(ctx, THRIFT_EXT_META_MAGIC)
	if err != nil {
		logger.Warn(ctx, "write thrift extension failed", "field", "magic", "err", err)
		return err
	}
	//ver
	err = p.WriteI16(ctx, THRIFT_EXT_META_VERSION)
	if err != nil {
		logger.Warn(ctx, "write thrift extension failed", "field", "version", "err", err)
		return err
	}
	//ext data
	nCtx := ctx.(*ExtContext)
	err = p.WriteMapBegin(ctx, thrift.STRING, thrift.STRING, len(nCtx.ReqExtData))
	if err != nil {
		logger.Warn(ctx, "write thrift extension map failed", "stage", "begin", "err", err)
		return err
	}
	for k, v := range nCtx.ReqExtData {
		if err = p.WriteString(ctx, k); err != nil {
			logger.Warn(ctx, "write thrift extension map failed", "stage", "key", "key", k, "err", err)
			return err
		}
		if err = p.WriteString(ctx, v); err != nil {
			logger.Warn(ctx, "write thrift extension map failed", "stage", "value", "key", k, "value", v, "err", err)
			return err
		}
	}
	p.WriteMapEnd(ctx)
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
	logger.Debug(ctx, "read thrift extension version", "version", ver)
	return ver, true, nil
}

// func (p *ExtProcessor) ReadMetaMap(ctx context.Context, in, out thrift.TProtocol) (map[string]string, error) {
func (p *ExtProcessor) ReadMetaMap(ctx context.Context, in, out thrift.TProtocol) (context.Context, error) {
	//reqData := make(map[string]string)
	_, _, size, err := in.ReadMapBegin(ctx)
	if err != nil {
		logger.Warn(ctx, "read thrift extension map failed", "stage", "header", "err", err)
		//return ctx, reqData, err
		return ctx, err
	}
	logger.Debug(ctx, "read thrift extension map", "size", size)
	var foundDepth bool = false
	var callClientService string = ""
	for i := 0; i < size; i++ {
		k, err := in.ReadString(ctx)
		if err != nil {
			logger.Warn(ctx, "read thrift extension map failed", "stage", "key", "index", i, "err", err)
			//return ctx, reqData, err
			return ctx, err
		}
		v, err := in.ReadString(ctx)
		if err != nil {
			logger.Warn(ctx, "read thrift extension map failed", "stage", "value", "index", i, "err", err)
			//return ctx, reqData, err
			return ctx, err
		}
		//reqData[k] = v
		if k == THRIFT_EXT_DEPTH {
			iv, err := strconv.Atoi(v)
			if err != nil {
				logger.Warn(ctx, "thrift_ext", "err", err)
			} else {
				foundDepth = true
				iv += 1
				ctx = context.WithValue(ctx, k, strconv.Itoa(iv))
			}
		} else if k == THRIFT_EXT_CALL_CLIENT_SERVICE {
			//ctx = context.WithValue(ctx, "client_service", v)
			callClientService = v
		} else {
			ctx = context.WithValue(ctx, k, v)
		}
		//logger.Debug(ctx, "thrift_ext", "k", k, "v", v)
	}
	if !foundDepth {
		ctx = context.WithValue(ctx, THRIFT_EXT_DEPTH, "0")
	}
	if callClientService != "" {
		ctx = context.WithValue(ctx, THRIFT_EXT_CLIENT_SERVICE, callClientService)
		logger.Debug(ctx, "thrift_ext", "callCleintService", callClientService)
	}
	logger.Debug(ctx, "thrift_ext", "test", "ccc")
	return ctx, err
}

func (p *ExtProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	//preBuf := make([]byte, 2)
	preBuf := make([]byte, 4)
	//starttime := time.Now()
	_, err := io.ReadFull(in.Transport(), preBuf)
	if err != nil {
		if err.(thrift.TProtocolException).TypeId() == thrift.END_OF_FILE {
			return false, thrift.NewTTransportException(thrift.END_OF_FILE, "connection closed (EOF)")
		} else {
			if err.(thrift.TProtocolException).TypeId() != thrift.TIMED_OUT {
				logger.Warn(ctx, "read thrift protocol prefix failed", "err", err)
			} else {
				logger.Debug(ctx, "read thrift protocol prefix timed out", "err", err)
			}
			return false, thrift.NewTTransportException(thrift.INVALID_DATA, "invalid data")
		}
	}
	//magic
	//magic := binary.BigEndian.Uint16(preBuf)
	magic := binary.BigEndian.Uint32(preBuf)
	//if magic == uint16(THRIFT_EXT_META_MAGIC) {
	if magic == uint32(THRIFT_EXT_META_MAGIC) {
		//if magic == THRIFT_EXT_META_MAGIC {
		logger.Debug(ctx, "select thrift protocol", "protocol", "extended")
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
				logger.Warn(ctx, "read thrift extension metadata failed", "err", errRM)
				return false, thrift.NewTTransportException(thrift.INVALID_DATA, "invalid data")
			}
		}
		/*for k, v := range reqData {
			ctx = context.WithValue(ctx, k, v)
		}*/

	} else {
		logger.Debug(ctx, "select thrift protocol", "protocol", "standard")
		multiReader := io.MultiReader(bytes.NewReader(preBuf), in.Transport())
		newTransport := thrift.NewStreamTransportR(multiReader)
		in = p.Pro.GetProtocol(newTransport)
		ctx = context.WithValue(ctx, "request_id", util.NewRequestID())
	}
	//b, t := p.Processor.Process(newCtx, in, out)
	logger.Debug(ctx, "thrift_ext", "test", "cccccc")
	b, t := p.Processor.Process(ctx, in, out)
	return b, t
}
