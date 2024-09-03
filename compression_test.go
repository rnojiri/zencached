package zencached_test

import (
	"encoding/base64"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/rnojiri/zencached"
	"github.com/stretchr/testify/assert"
)

func TestBase64Encoding(t *testing.T) {

	dcs, err := zencached.NewDataCompressor(zencached.CompressionTypeBase64, 0)
	assert.NoError(t, err, "expects no error creating the base64 encoder")

	notEncoded := gofakeit.BeerName()
	expectedEncoded := base64.RawStdEncoding.EncodeToString([]byte(notEncoded))

	result, err := dcs.Compress([]byte(notEncoded))
	assert.NoError(t, err, "expects no error encoding in base64")

	assert.Equal(t, expectedEncoded, string(result), "expects same encoded results in base64")
}

func TestBase64Decoding(t *testing.T) {

	dcs, err := zencached.NewDataCompressor(zencached.CompressionTypeBase64, 0)
	assert.NoError(t, err, "expects no error creating the base64 encoder")

	notEncoded := gofakeit.Email()
	encoded := base64.RawStdEncoding.EncodeToString([]byte(notEncoded))

	result, err := dcs.Decompress([]byte(encoded))
	assert.NoError(t, err, "expects no error decoding in base64")

	assert.Equal(t, notEncoded, string(result), "expects same decoded results from base64")
}

func TestZstandardCompression(t *testing.T) {

	dcs, err := zencached.NewDataCompressor(zencached.CompressionTypeZstandard, 10)
	assert.NoError(t, err, "expects no error creating the zstandard compressor")

	notCompressed := gofakeit.HipsterSentence(10000)
	compressedData, err := dcs.Compress([]byte(notCompressed))
	assert.NoError(t, err, "expects no error compressing in zstandard")

	assert.LessOrEqual(t, len(compressedData), len(notCompressed), "expected the compressed data to be less than the original data")
	assert.LessOrEqual(t, float64(70), (1-float64(len(compressedData))/float64(len(notCompressed)))*100, "expected the compressed data to be less than the original data")

	decompressedData, err := dcs.Decompress(compressedData)
	assert.NoError(t, err, "expects no error decompressing from zstandard")

	assert.Equal(t, notCompressed, string(decompressedData), "expects the decompressed data to be equals to the original one")
}
