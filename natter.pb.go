// Code generated by protoc-gen-go. DO NOT EDIT.
// source: natter.proto

package natter

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// 0x01
type CheckinRequest struct {
	Source               string   `protobuf:"bytes,1,opt,name=Source,proto3" json:"Source,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CheckinRequest) Reset()         { *m = CheckinRequest{} }
func (m *CheckinRequest) String() string { return proto.CompactTextString(m) }
func (*CheckinRequest) ProtoMessage()    {}
func (*CheckinRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_39129eec69c18084, []int{0}
}

func (m *CheckinRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CheckinRequest.Unmarshal(m, b)
}
func (m *CheckinRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CheckinRequest.Marshal(b, m, deterministic)
}
func (m *CheckinRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CheckinRequest.Merge(m, src)
}
func (m *CheckinRequest) XXX_Size() int {
	return xxx_messageInfo_CheckinRequest.Size(m)
}
func (m *CheckinRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CheckinRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CheckinRequest proto.InternalMessageInfo

func (m *CheckinRequest) GetSource() string {
	if m != nil {
		return m.Source
	}
	return ""
}

// 0x02
type CheckinResponse struct {
	Addr                 string   `protobuf:"bytes,1,opt,name=Addr,proto3" json:"Addr,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CheckinResponse) Reset()         { *m = CheckinResponse{} }
func (m *CheckinResponse) String() string { return proto.CompactTextString(m) }
func (*CheckinResponse) ProtoMessage()    {}
func (*CheckinResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_39129eec69c18084, []int{1}
}

func (m *CheckinResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CheckinResponse.Unmarshal(m, b)
}
func (m *CheckinResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CheckinResponse.Marshal(b, m, deterministic)
}
func (m *CheckinResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CheckinResponse.Merge(m, src)
}
func (m *CheckinResponse) XXX_Size() int {
	return xxx_messageInfo_CheckinResponse.Size(m)
}
func (m *CheckinResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CheckinResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CheckinResponse proto.InternalMessageInfo

func (m *CheckinResponse) GetAddr() string {
	if m != nil {
		return m.Addr
	}
	return ""
}

// 0x03
type ForwardRequest struct {
	Id                   string   `protobuf:"bytes,1,opt,name=Id,proto3" json:"Id,omitempty"`
	Source               string   `protobuf:"bytes,2,opt,name=Source,proto3" json:"Source,omitempty"`
	SourceAddr           string   `protobuf:"bytes,3,opt,name=SourceAddr,proto3" json:"SourceAddr,omitempty"`
	Target               string   `protobuf:"bytes,4,opt,name=Target,proto3" json:"Target,omitempty"`
	TargetAddr           string   `protobuf:"bytes,5,opt,name=TargetAddr,proto3" json:"TargetAddr,omitempty"`
	TargetForwardAddr    string   `protobuf:"bytes,6,opt,name=TargetForwardAddr,proto3" json:"TargetForwardAddr,omitempty"`
	TargetCommand        []string `protobuf:"bytes,7,rep,name=TargetCommand,proto3" json:"TargetCommand,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ForwardRequest) Reset()         { *m = ForwardRequest{} }
func (m *ForwardRequest) String() string { return proto.CompactTextString(m) }
func (*ForwardRequest) ProtoMessage()    {}
func (*ForwardRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_39129eec69c18084, []int{2}
}

func (m *ForwardRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ForwardRequest.Unmarshal(m, b)
}
func (m *ForwardRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ForwardRequest.Marshal(b, m, deterministic)
}
func (m *ForwardRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ForwardRequest.Merge(m, src)
}
func (m *ForwardRequest) XXX_Size() int {
	return xxx_messageInfo_ForwardRequest.Size(m)
}
func (m *ForwardRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ForwardRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ForwardRequest proto.InternalMessageInfo

func (m *ForwardRequest) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *ForwardRequest) GetSource() string {
	if m != nil {
		return m.Source
	}
	return ""
}

func (m *ForwardRequest) GetSourceAddr() string {
	if m != nil {
		return m.SourceAddr
	}
	return ""
}

func (m *ForwardRequest) GetTarget() string {
	if m != nil {
		return m.Target
	}
	return ""
}

func (m *ForwardRequest) GetTargetAddr() string {
	if m != nil {
		return m.TargetAddr
	}
	return ""
}

func (m *ForwardRequest) GetTargetForwardAddr() string {
	if m != nil {
		return m.TargetForwardAddr
	}
	return ""
}

func (m *ForwardRequest) GetTargetCommand() []string {
	if m != nil {
		return m.TargetCommand
	}
	return nil
}

// 0x04
type ForwardResponse struct {
	Id                   string   `protobuf:"bytes,1,opt,name=Id,proto3" json:"Id,omitempty"`
	Success              bool     `protobuf:"varint,2,opt,name=Success,proto3" json:"Success,omitempty"`
	Source               string   `protobuf:"bytes,3,opt,name=Source,proto3" json:"Source,omitempty"`
	SourceAddr           string   `protobuf:"bytes,4,opt,name=SourceAddr,proto3" json:"SourceAddr,omitempty"`
	Target               string   `protobuf:"bytes,5,opt,name=Target,proto3" json:"Target,omitempty"`
	TargetAddr           string   `protobuf:"bytes,6,opt,name=TargetAddr,proto3" json:"TargetAddr,omitempty"`
	TargetCommand        string   `protobuf:"bytes,7,opt,name=TargetCommand,proto3" json:"TargetCommand,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ForwardResponse) Reset()         { *m = ForwardResponse{} }
func (m *ForwardResponse) String() string { return proto.CompactTextString(m) }
func (*ForwardResponse) ProtoMessage()    {}
func (*ForwardResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_39129eec69c18084, []int{3}
}

func (m *ForwardResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ForwardResponse.Unmarshal(m, b)
}
func (m *ForwardResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ForwardResponse.Marshal(b, m, deterministic)
}
func (m *ForwardResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ForwardResponse.Merge(m, src)
}
func (m *ForwardResponse) XXX_Size() int {
	return xxx_messageInfo_ForwardResponse.Size(m)
}
func (m *ForwardResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ForwardResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ForwardResponse proto.InternalMessageInfo

func (m *ForwardResponse) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *ForwardResponse) GetSuccess() bool {
	if m != nil {
		return m.Success
	}
	return false
}

func (m *ForwardResponse) GetSource() string {
	if m != nil {
		return m.Source
	}
	return ""
}

func (m *ForwardResponse) GetSourceAddr() string {
	if m != nil {
		return m.SourceAddr
	}
	return ""
}

func (m *ForwardResponse) GetTarget() string {
	if m != nil {
		return m.Target
	}
	return ""
}

func (m *ForwardResponse) GetTargetAddr() string {
	if m != nil {
		return m.TargetAddr
	}
	return ""
}

func (m *ForwardResponse) GetTargetCommand() string {
	if m != nil {
		return m.TargetCommand
	}
	return ""
}

func init() {
	proto.RegisterType((*CheckinRequest)(nil), "natter.CheckinRequest")
	proto.RegisterType((*CheckinResponse)(nil), "natter.CheckinResponse")
	proto.RegisterType((*ForwardRequest)(nil), "natter.ForwardRequest")
	proto.RegisterType((*ForwardResponse)(nil), "natter.ForwardResponse")
}

func init() { proto.RegisterFile("natter.proto", fileDescriptor_39129eec69c18084) }

var fileDescriptor_39129eec69c18084 = []byte{
	// 261 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x52, 0xc1, 0x4a, 0xc3, 0x40,
	0x10, 0x65, 0x93, 0x74, 0x6b, 0x07, 0x4d, 0x71, 0x0f, 0x92, 0x93, 0x94, 0xa0, 0x90, 0x83, 0x78,
	0xf1, 0x0b, 0xa4, 0x20, 0xf4, 0x9a, 0xfa, 0x03, 0x31, 0x3b, 0xa8, 0x48, 0xb3, 0x75, 0x77, 0x83,
	0xdf, 0xe9, 0x3f, 0xf8, 0x21, 0xd2, 0x99, 0x6d, 0x5c, 0x8d, 0xe4, 0x36, 0xef, 0xed, 0x7b, 0x33,
	0x79, 0x8f, 0xc0, 0x69, 0xd7, 0x78, 0x8f, 0xf6, 0x76, 0x6f, 0x8d, 0x37, 0x4a, 0x32, 0x2a, 0x2b,
	0xc8, 0xd7, 0x2f, 0xd8, 0xbe, 0xbd, 0x76, 0x35, 0xbe, 0xf7, 0xe8, 0xbc, 0xba, 0x00, 0xb9, 0x35,
	0xbd, 0x6d, 0xb1, 0x10, 0x2b, 0x51, 0x2d, 0xea, 0x80, 0xca, 0x6b, 0x58, 0x0e, 0x4a, 0xb7, 0x37,
	0x9d, 0x43, 0xa5, 0x20, 0xbb, 0xd7, 0xda, 0x06, 0x21, 0xcd, 0xe5, 0x97, 0x80, 0xfc, 0xc1, 0xd8,
	0x8f, 0xc6, 0xea, 0xe3, 0xc6, 0x1c, 0x92, 0x8d, 0x0e, 0xa2, 0x64, 0xa3, 0xa3, 0x0b, 0x49, 0x7c,
	0x41, 0x5d, 0x02, 0xf0, 0x44, 0x4b, 0x53, 0x7a, 0x8b, 0x98, 0x83, 0xef, 0xb1, 0xb1, 0xcf, 0xe8,
	0x8b, 0x8c, 0x7d, 0x8c, 0x0e, 0x3e, 0x9e, 0xc8, 0x37, 0x63, 0xdf, 0x0f, 0xa3, 0x6e, 0xe0, 0x9c,
	0x51, 0xf8, 0x2e, 0x92, 0x49, 0x92, 0x8d, 0x1f, 0xd4, 0x15, 0x9c, 0x31, 0xb9, 0x36, 0xbb, 0x5d,
	0xd3, 0xe9, 0x62, 0xbe, 0x4a, 0xab, 0x45, 0xfd, 0x9b, 0x2c, 0x3f, 0x05, 0x2c, 0x87, 0x98, 0xa1,
	0x8e, 0xbf, 0x39, 0x0b, 0x98, 0x6f, 0xfb, 0xb6, 0x45, 0xe7, 0x28, 0xe8, 0x49, 0x7d, 0x84, 0x51,
	0x03, 0xe9, 0x44, 0x03, 0xd9, 0x44, 0x03, 0xb3, 0x89, 0x06, 0xe4, 0xa8, 0x81, 0x7f, 0x32, 0x89,
	0x51, 0xa6, 0x27, 0x49, 0xbf, 0xc6, 0xdd, 0x77, 0x00, 0x00, 0x00, 0xff, 0xff, 0x5c, 0x49, 0xc6,
	0x00, 0x2a, 0x02, 0x00, 0x00,
}
