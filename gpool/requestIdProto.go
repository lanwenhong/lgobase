package gpool

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/logger"
)

type RequestIDProtocolClient struct {
	thrift.TProtocol
	baseFactory thrift.TProtocolFactory
}

type RequestIDProcessor struct {
	thrift.TProcessor
	Processor thrift.TProcessor
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

func NewRequestIDProtocolClient(baseFactory thrift.TProtocolFactory) *RequestIDProtocolClient {
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
}

func (p *RequestIDProtocolClient) WriteMessageBegin(ctx context.Context, name string, typeId thrift.TMessageType, seqId int32) error {
	rid := GetRequestID(ctx)
	logger.Debugf(ctx, "rid: %s", rid)
	if rid != "" {
		logger.Debugf(ctx, "rid: %s", rid)
		if err := p.WriteString(ctx, rid); err != nil {
			return err
		}
	}
	return p.TProtocol.WriteMessageBegin(ctx, name, typeId, seqId)
}

func NewRequestIDProcessor(processor thrift.TProcessor, p *thrift.TBinaryProtocolFactory) *RequestIDProcessor {
	return &RequestIDProcessor{
		Processor: processor,
	}
}

func (p *RequestIDProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	rid, err := in.ReadString(ctx)
	if err != nil {
		if err.(thrift.TProtocolException).TypeId() == thrift.END_OF_FILE {
			return false, thrift.NewTTransportException(thrift.END_OF_FILE, "connection closed (EOF)")
		} else {
			logger.Warnf(ctx, "read request_id err: %v", err)
			return false, thrift.NewTTransportException(thrift.INVALID_DATA, "invalid data")
		}
	}
	logger.Debugf(ctx, "rid: %s", rid)
	newCtx := WithRequestID(ctx, rid)
	b, t := p.Processor.Process(newCtx, in, out)
	return b, t
}
