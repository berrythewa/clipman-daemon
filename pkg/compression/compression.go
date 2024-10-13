package compression

import (
    "bytes"
    "compress/gzip"
    "encoding/base64"
    "io/ioutil"

    "github.com/berrythewa/clipman-daemon/internal/types"
)

const compressionThreshold = 1024 // 1KB

func compressContent(content *types.ClipboardContent) (*types.ClipboardContent, error) {
    if len(content.Data) < compressionThreshold {
        return content, nil
    }

    var buf bytes.Buffer
    zw := gzip.NewWriter(&buf)
    _, err := zw.Write(content.Data) // Data is now []byte
    if err != nil {
        return nil, err
    }
    if err := zw.Close(); err != nil {
        return nil, err
    }

    compressedData := base64.StdEncoding.EncodeToString(buf.Bytes())

    return &types.ClipboardContent{
        Type:       content.Type,
        Data:       []byte(compressedData), // Store the base64-encoded data as []byte
        Created:    content.Created,
        Compressed: true,
    }, nil
}

func DecompressContent(content *types.ClipboardContent) (*types.ClipboardContent, error) {
    if !content.Compressed {
        return content, nil
    }

    decoded, err := base64.StdEncoding.DecodeString(string(content.Data)) // Decode the base64-encoded []byte
    if err != nil {
        return nil, err
    }

    zr, err := gzip.NewReader(bytes.NewReader(decoded))
    if err != nil {
        return nil, err
    }
    defer zr.Close()

    uncompressed, err := ioutil.ReadAll(zr)
    if err != nil {
        return nil, err
    }

    return &types.ClipboardContent{
        Type:       content.Type,
        Data:       uncompressed, // Use uncompressed []byte directly
        Created:    content.Created,
        Compressed: false,
    }, nil
}
