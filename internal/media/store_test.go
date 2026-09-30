package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	webpencoder "github.com/gen2brain/webp"
	"golang.org/x/image/webp"
)

func TestReadImageConvertsJPEGAndPNGToWebP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		encode func(*bytes.Buffer, image.Image) error
	}{
		{name: "JPEG", encode: func(dst *bytes.Buffer, src image.Image) error { return jpeg.Encode(dst, src, nil) }},
		{name: "PNG", encode: func(dst *bytes.Buffer, src image.Image) error { return png.Encode(dst, src) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(0, 0, 16, 12))
			for y := 0; y < 12; y++ {
				for x := 0; x < 16; x++ {
					src.SetNRGBA(x, y, color.NRGBA{R: 220, G: 70, B: 20, A: 255})
				}
			}
			var input bytes.Buffer
			if err := tc.encode(&input, src); err != nil {
				t.Fatal(err)
			}
			output, info, err := readImage(bytes.NewReader(input.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if info.MIMEType != "image/webp" || info.Extension != ".webp" || info.Width != 16 || info.Height != 12 {
				t.Fatalf("转换后图片信息不正确：%+v", info)
			}
			decoded, err := webp.Decode(bytes.NewReader(output))
			if err != nil {
				t.Fatalf("保存内容不是有效 WebP：%v", err)
			}
			if decoded.Bounds().Dx() != 16 || decoded.Bounds().Dy() != 12 {
				t.Fatalf("转换后尺寸不正确：%v", decoded.Bounds())
			}
		})
	}
}

func TestReadImagePreservesPNGTransparency(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 0})
	src.SetNRGBA(1, 0, color.NRGBA{R: 255, A: 255})
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	output, _, err := readImage(bytes.NewReader(input.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := webp.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, transparentAlpha := decoded.At(0, 0).RGBA()
	_, _, _, opaqueAlpha := decoded.At(1, 0).RGBA()
	if transparentAlpha != 0 || opaqueAlpha != 0xffff {
		t.Fatalf("透明度丢失：透明像素=%d，不透明像素=%d", transparentAlpha, opaqueAlpha)
	}
}

func TestReadImageAppliesJPEGEXIFOrientation(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 12))
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, src, nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{
		'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 0x2a, 0, 8, 0, 0, 0,
		1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0,
	}
	input := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	input = append(input, plain.Bytes()[2:]...)
	validated, err := Validate(input)
	if err != nil {
		t.Fatal(err)
	}
	if validated.Width != 12 || validated.Height != 16 {
		t.Fatalf("校验结果未应用 EXIF 方向：%+v", validated)
	}
	output, info, err := readImage(bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := webp.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 12 || info.Height != 16 || decoded.Bounds().Dx() != 12 || decoded.Bounds().Dy() != 16 {
		t.Fatalf("未应用 EXIF 方向：info=%+v，输出尺寸=%v", info, decoded.Bounds())
	}
}

func TestReadImageKeepsExistingWebPBytes(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	src.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	var input bytes.Buffer
	if err := webpencoder.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	output, info, err := readImage(bytes.NewReader(input.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, input.Bytes()) || info.MIMEType != "image/webp" || info.Extension != ".webp" {
		t.Fatalf("现有 WebP 应原样保存：info=%+v", info)
	}
}

func TestOrientImageMapsAllEXIFOrientations(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(y*2 + x + 1), A: 255})
		}
	}
	wants := map[int][][]uint8{
		2: {{2, 1}, {4, 3}, {6, 5}},
		3: {{6, 5}, {4, 3}, {2, 1}},
		4: {{5, 6}, {3, 4}, {1, 2}},
		5: {{1, 3, 5}, {2, 4, 6}},
		6: {{5, 3, 1}, {6, 4, 2}},
		7: {{6, 4, 2}, {5, 3, 1}},
		8: {{2, 4, 6}, {1, 3, 5}},
	}
	for orientation, rows := range wants {
		oriented := orientImage(source, orientation)
		if got := oriented.Bounds().Dy(); got != len(rows) {
			t.Fatalf("方向 %d 高度=%d，期望 %d", orientation, got, len(rows))
		}
		for y, row := range rows {
			if got := oriented.Bounds().Dx(); got != len(row) {
				t.Fatalf("方向 %d 宽度=%d，期望 %d", orientation, got, len(row))
			}
			for x, want := range row {
				red, _, _, _ := oriented.At(x, y).RGBA()
				if got := uint8(red >> 8); got != want {
					t.Fatalf("方向 %d 的像素 (%d,%d)=%d，期望 %d", orientation, x, y, got, want)
				}
			}
		}
	}
}
