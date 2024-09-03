package zencached

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

var ErrUnknownCompressionType error = errors.New("unknown compression type")

// DataCompressor - compression implementation
type DataCompressor interface {

	// Compress - compress the data
	Compress([]byte) ([]byte, error)

	// Decompress - decompress the data
	Decompress([]byte) ([]byte, error)
}

// NewDataCompressor - creates a new data copressor implementation
func NewDataCompressor(ctype CompressionType, compressionLevel int) (DataCompressor, error) {

	switch ctype {
	case CompressionTypeBase64:
		return base64Encoder{}, nil
	case CompressionTypeZstandard:
		return zstdCompression{
			compressionLevel: zstd.EncoderLevelFromZstd(compressionLevel),
		}, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownCompressionType, ctype.String())
	}
}

type base64Encoder struct{}

func (dc base64Encoder) Compress(data []byte) ([]byte, error) {

	dst := make([]byte, base64.RawStdEncoding.EncodedLen(len(data)))

	base64.RawStdEncoding.Encode(dst, data)

	return dst, nil
}

func (dc base64Encoder) Decompress(data []byte) ([]byte, error) {

	dst := make([]byte, base64.RawStdEncoding.DecodedLen(len(data)))

	index, err := base64.RawStdEncoding.Decode(dst, data)
	if err != nil {
		return nil, err
	}

	return dst[:index], nil
}

var _ DataCompressor = (*base64Encoder)(nil)

type zstdCompression struct {
	compressionLevel zstd.EncoderLevel
}

func (dc zstdCompression) Compress(data []byte) ([]byte, error) {

	var tmpBuffer bytes.Buffer

	encoder, err := zstd.NewWriter(
		&tmpBuffer,
		zstd.WithEncoderLevel(dc.compressionLevel),
	)
	if err != nil {
		return nil, err
	}

	_, err = encoder.Write(data)
	if err != nil {
		return nil, err
	}

	if err = encoder.Close(); err != nil {
		return nil, err
	}

	return tmpBuffer.Bytes(), nil
}

func (dc zstdCompression) Decompress(data []byte) ([]byte, error) {

	readerBuffer := bytes.NewReader(data)

	decoder, err := zstd.NewReader(readerBuffer)
	if err != nil {
		return nil, err
	}

	defer decoder.Close()

	var decompressed bytes.Buffer

	_, err = io.Copy(&decompressed, decoder)
	if err != nil {
		return nil, err
	}

	return decompressed.Bytes(), nil
}

var _ DataCompressor = (*base64Encoder)(nil)
