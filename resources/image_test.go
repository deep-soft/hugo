// Copyright 2019 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources_test

import (
	"context"
	"fmt"
	"image"
	"image/gif"
	"io/fs"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gohugoio/hugo/htesting"
	"github.com/gohugoio/hugo/resources/images/webp"

	"github.com/gohugoio/hugo/common/paths"

	"github.com/spf13/afero"

	"github.com/disintegration/gift"

	"github.com/gohugoio/hugo/helpers"

	"github.com/gohugoio/hugo/media"
	"github.com/gohugoio/hugo/resources/images"
	"github.com/google/go-cmp/cmp"

	"github.com/gohugoio/hugo/htesting/hqt"

	qt "github.com/frankban/quicktest"
)

var eq = qt.CmpEquals(
	cmp.Comparer(func(p1, p2 os.FileInfo) bool {
		return p1.Name() == p2.Name() && p1.Size() == p2.Size() && p1.IsDir() == p2.IsDir()
	}),
	cmp.Comparer(func(d1, d2 fs.DirEntry) bool {
		p1, err1 := d1.Info()
		p2, err2 := d2.Info()
		if err1 != nil || err2 != nil {
			return false
		}
		return p1.Name() == p2.Name() && p1.Size() == p2.Size() && p1.IsDir() == p2.IsDir()
	}),
	// cmp.Comparer(func(p1, p2 *genericResource) bool { return p1 == p2 }),
	cmp.Comparer(func(m1, m2 media.Type) bool {
		return m1.Type == m2.Type
	}),
	cmp.Comparer(
		func(v1, v2 *big.Rat) bool {
			return v1.RatString() == v2.RatString()
		},
	),
	cmp.Comparer(func(v1, v2 time.Time) bool {
		return v1.Unix() == v2.Unix()
	}),
)

func TestImageTransformBasic(t *testing.T) {
	c := qt.New(t)

	_, image := fetchSunset(c)

	assertWidthHeight := func(img images.ImageResource, w, h int) {
		assertWidthHeight(c, img, w, h)
	}

	gotColors, err := image.Colors()
	c.Assert(err, qt.IsNil)
	expectedColors := images.HexStringsToColors("#2d2f33", "#a49e93", "#d39e59", "#a76936", "#737a84", "#7c838b")
	c.Assert(len(gotColors), qt.Equals, len(expectedColors))
	for i := range gotColors {
		c1, c2 := gotColors[i], expectedColors[i]
		c.Assert(c1.ColorHex(), qt.Equals, c2.ColorHex())
		c.Assert(c1.ColorGo(), qt.DeepEquals, c2.ColorGo())
		c.Assert(c1.Luminance(), qt.Equals, c2.Luminance())
	}

	c.Assert(image.RelPermalink(), qt.Equals, "/a/sunset.jpg")
	c.Assert(image.ResourceType(), qt.Equals, "image")
	assertWidthHeight(image, 900, 562)

	resized, err := image.Resize("300x200")
	c.Assert(err, qt.IsNil)
	c.Assert(image != resized, qt.Equals, true)
	assertWidthHeight(resized, 300, 200)
	assertWidthHeight(image, 900, 562)

	resized0x, err := image.Resize("x200")
	c.Assert(err, qt.IsNil)
	assertWidthHeight(resized0x, 320, 200)

	resizedx0, err := image.Resize("200x")
	c.Assert(err, qt.IsNil)
	assertWidthHeight(resizedx0, 200, 125)

	resizedAndRotated, err := image.Resize("x200 r90")
	c.Assert(err, qt.IsNil)
	assertWidthHeight(resizedAndRotated, 125, 200)

	assertWidthHeight(resized, 300, 200)
	c.Assert(resized.RelPermalink(), qt.Equals, "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_300x200_resize_q68_linear.jpg")

	fitted, err := resized.Fit("50x50")
	c.Assert(err, qt.IsNil)
	c.Assert(fitted.RelPermalink(), qt.Equals, "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_625708021e2bb281c9f1002f88e4753f.jpg")
	assertWidthHeight(fitted, 50, 33)

	// Check the MD5 key threshold
	fittedAgain, _ := fitted.Fit("10x20")
	fittedAgain, err = fittedAgain.Fit("10x20")
	c.Assert(err, qt.IsNil)
	c.Assert(fittedAgain.RelPermalink(), qt.Equals, "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_3f65ba24dc2b7fba0f56d7f104519157.jpg")
	assertWidthHeight(fittedAgain, 10, 7)

	filled, err := image.Fill("200x100 bottomLeft")
	c.Assert(err, qt.IsNil)
	c.Assert(filled.RelPermalink(), qt.Equals, "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_200x100_fill_q68_linear_bottomleft.jpg")
	assertWidthHeight(filled, 200, 100)

	smart, err := image.Fill("200x100 smart")
	c.Assert(err, qt.IsNil)
	c.Assert(smart.RelPermalink(), qt.Equals, fmt.Sprintf("/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_200x100_fill_q68_linear_smart%d.jpg", 1))
	assertWidthHeight(smart, 200, 100)

	// Check cache
	filledAgain, err := image.Fill("200x100 bottomLeft")
	c.Assert(err, qt.IsNil)
	c.Assert(filled, qt.Equals, filledAgain)

	cropped, err := image.Crop("300x300 topRight")
	c.Assert(err, qt.IsNil)
	c.Assert(cropped.RelPermalink(), qt.Equals, "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_300x300_crop_q68_linear_topright.jpg")
	assertWidthHeight(cropped, 300, 300)

	smartcropped, err := image.Crop("200x200 smart")
	c.Assert(err, qt.IsNil)
	c.Assert(smartcropped.RelPermalink(), qt.Equals, fmt.Sprintf("/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_200x200_crop_q68_linear_smart%d.jpg", 1))
	assertWidthHeight(smartcropped, 200, 200)

	// Check cache
	croppedAgain, err := image.Crop("300x300 topRight")
	c.Assert(err, qt.IsNil)
	c.Assert(cropped, qt.Equals, croppedAgain)
}

func TestImageProcess(t *testing.T) {
	c := qt.New(t)
	_, img := fetchSunset(c)
	resized, err := img.Process("resiZe 300x200")
	c.Assert(err, qt.IsNil)
	assertWidthHeight(c, resized, 300, 200)
	rotated, err := resized.Process("R90")
	c.Assert(err, qt.IsNil)
	assertWidthHeight(c, rotated, 200, 300)
	converted, err := img.Process("png")
	c.Assert(err, qt.IsNil)
	c.Assert(converted.MediaType().Type, qt.Equals, "image/png")

	checkProcessVsMethod := func(action, spec string) {
		var expect images.ImageResource
		var err error
		switch action {
		case images.ActionCrop:
			expect, err = img.Crop(spec)
		case images.ActionFill:
			expect, err = img.Fill(spec)
		case images.ActionFit:
			expect, err = img.Fit(spec)
		case images.ActionResize:
			expect, err = img.Resize(spec)
		}
		c.Assert(err, qt.IsNil)
		got, err := img.Process(spec + " " + action)
		c.Assert(err, qt.IsNil)
		assertWidthHeight(c, got, expect.Width(), expect.Height())
		c.Assert(got.MediaType(), qt.Equals, expect.MediaType())
	}

	checkProcessVsMethod(images.ActionCrop, "300x200 topleFt")
	checkProcessVsMethod(images.ActionFill, "300x200 topleft")
	checkProcessVsMethod(images.ActionFit, "300x200 png")
	checkProcessVsMethod(images.ActionResize, "300x R90")
}

func TestImageTransformFormat(t *testing.T) {
	c := qt.New(t)

	_, image := fetchSunset(c)

	assertExtWidthHeight := func(img images.ImageResource, ext string, w, h int) {
		c.Helper()
		c.Assert(img, qt.Not(qt.IsNil))
		c.Assert(paths.Ext(img.RelPermalink()), qt.Equals, ext)
		c.Assert(img.Width(), qt.Equals, w)
		c.Assert(img.Height(), qt.Equals, h)
	}

	c.Assert(image.RelPermalink(), qt.Equals, "/a/sunset.jpg")
	c.Assert(image.ResourceType(), qt.Equals, "image")
	assertExtWidthHeight(image, ".jpg", 900, 562)

	imagePng, err := image.Resize("450x png")
	c.Assert(err, qt.IsNil)
	c.Assert(imagePng.RelPermalink(), qt.Equals, "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_450x0_resize_linear.png")
	c.Assert(imagePng.ResourceType(), qt.Equals, "image")
	assertExtWidthHeight(imagePng, ".png", 450, 281)
	c.Assert(imagePng.Name(), qt.Equals, "sunset.jpg")
	c.Assert(imagePng.MediaType().String(), qt.Equals, "image/png")

	imageGif, err := image.Resize("225x gif")
	c.Assert(err, qt.IsNil)
	c.Assert(imageGif.RelPermalink(), qt.Equals, "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_225x0_resize_linear.gif")
	c.Assert(imageGif.ResourceType(), qt.Equals, "image")
	assertExtWidthHeight(imageGif, ".gif", 225, 141)
	c.Assert(imageGif.Name(), qt.Equals, "sunset.jpg")
	c.Assert(imageGif.MediaType().String(), qt.Equals, "image/gif")
}

// https://github.com/gohugoio/hugo/issues/5730
func TestImagePermalinkPublishOrder(t *testing.T) {
	for _, checkOriginalFirst := range []bool{true, false} {
		name := "OriginalFirst"
		if !checkOriginalFirst {
			name = "ResizedFirst"
		}

		t.Run(name, func(t *testing.T) {
			c := qt.New(t)
			spec, workDir := newTestResourceOsFs(c)
			defer func() {
				os.Remove(workDir)
			}()

			check1 := func(img images.ImageResource) {
				resizedLink := "/a/sunset_hu59e56ffff1bc1d8d122b1403d34e039f_90587_100x50_resize_q75_box.jpg"
				c.Assert(img.RelPermalink(), qt.Equals, resizedLink)
				assertImageFile(c, spec.PublishFs, resizedLink, 100, 50)
			}

			check2 := func(img images.ImageResource) {
				c.Assert(img.RelPermalink(), qt.Equals, "/a/sunset.jpg")
				assertImageFile(c, spec.PublishFs, "a/sunset.jpg", 900, 562)
			}

			original := fetchImageForSpec(spec, c, "sunset.jpg")
			c.Assert(original, qt.Not(qt.IsNil))

			if checkOriginalFirst {
				check2(original)
			}

			resized, err := original.Resize("100x50")
			c.Assert(err, qt.IsNil)

			check1(resized)

			if !checkOriginalFirst {
				check2(original)
			}
		})
	}
}

func TestImageBugs(t *testing.T) {
	c := qt.New(t)

	// Issue #4261
	c.Run("Transform long filename", func(c *qt.C) {
		_, image := fetchImage(c, "1234567890qwertyuiopasdfghjklzxcvbnm5to6eeeeee7via8eleph.jpg")
		c.Assert(image, qt.Not(qt.IsNil))

		resized, err := image.Resize("200x")
		c.Assert(err, qt.IsNil)
		c.Assert(resized, qt.Not(qt.IsNil))
		c.Assert(resized.Width(), qt.Equals, 200)
		c.Assert(resized.RelPermalink(), qt.Equals, "/a/_hu59e56ffff1bc1d8d122b1403d34e039f_90587_65b757a6e14debeae720fe8831f0a9bc.jpg")
		resized, err = resized.Resize("100x")
		c.Assert(err, qt.IsNil)
		c.Assert(resized, qt.Not(qt.IsNil))
		c.Assert(resized.Width(), qt.Equals, 100)
		c.Assert(resized.RelPermalink(), qt.Equals, "/a/_hu59e56ffff1bc1d8d122b1403d34e039f_90587_c876768085288f41211f768147ba2647.jpg")
	})

	// Issue #6137
	c.Run("Transform upper case extension", func(c *qt.C) {
		_, image := fetchImage(c, "sunrise.JPG")

		resized, err := image.Resize("200x")
		c.Assert(err, qt.IsNil)
		c.Assert(resized, qt.Not(qt.IsNil))
		c.Assert(resized.Width(), qt.Equals, 200)
	})

	// Issue #7955
	c.Run("Fill with smartcrop", func(c *qt.C) {
		_, sunset := fetchImage(c, "sunset.jpg")

		for _, test := range []struct {
			originalDimensions string
			targetWH           int
		}{
			{"408x403", 400},
			{"425x403", 400},
			{"459x429", 400},
			{"476x442", 400},
			{"544x403", 400},
			{"476x468", 400},
			{"578x585", 550},
			{"578x598", 550},
		} {
			c.Run(test.originalDimensions, func(c *qt.C) {
				image, err := sunset.Resize(test.originalDimensions)
				c.Assert(err, qt.IsNil)
				resized, err := image.Fill(fmt.Sprintf("%dx%d smart", test.targetWH, test.targetWH))
				c.Assert(err, qt.IsNil)
				c.Assert(resized, qt.Not(qt.IsNil))
				c.Assert(resized.Width(), qt.Equals, test.targetWH)
				c.Assert(resized.Height(), qt.Equals, test.targetWH)
			})
		}
	})
}

func TestImageTransformConcurrent(t *testing.T) {
	var wg sync.WaitGroup

	c := qt.New(t)

	spec, workDir := newTestResourceOsFs(c)
	defer func() {
		os.Remove(workDir)
	}()

	image := fetchImageForSpec(spec, c, "sunset.jpg")

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				img := image
				for k := 0; k < 2; k++ {
					r1, err := img.Resize(fmt.Sprintf("%dx", id-k))
					if err != nil {
						t.Error(err)
					}

					if r1.Width() != id-k {
						t.Errorf("Width: %d:%d", r1.Width(), j)
					}

					r2, err := r1.Resize(fmt.Sprintf("%dx", id-k-1))
					if err != nil {
						t.Error(err)
					}

					img = r2
				}
			}
		}(i + 20)
	}

	wg.Wait()
}

func TestImageResize8BitPNG(t *testing.T) {
	c := qt.New(t)

	_, image := fetchImage(c, "gohugoio.png")

	c.Assert(image.MediaType().Type, qt.Equals, "image/png")
	c.Assert(image.RelPermalink(), qt.Equals, "/a/gohugoio.png")
	c.Assert(image.ResourceType(), qt.Equals, "image")
	c.Assert(image.Exif(), qt.IsNil)

	resized, err := image.Resize("800x")
	c.Assert(err, qt.IsNil)
	c.Assert(resized.MediaType().Type, qt.Equals, "image/png")
	c.Assert(resized.RelPermalink(), qt.Equals, "/a/gohugoio_hu0e1b9e4a4be4d6f86c7b37b9ccce3fbc_73886_800x0_resize_linear_3.png")
	c.Assert(resized.Width(), qt.Equals, 800)
}

func TestSVGImage(t *testing.T) {
	c := qt.New(t)
	spec := newTestResourceSpec(specDescriptor{c: c})
	svg := fetchResourceForSpec(spec, c, "circle.svg")
	c.Assert(svg, qt.Not(qt.IsNil))
}

func TestSVGImageContent(t *testing.T) {
	c := qt.New(t)
	spec := newTestResourceSpec(specDescriptor{c: c})
	svg := fetchResourceForSpec(spec, c, "circle.svg")
	c.Assert(svg, qt.Not(qt.IsNil))

	content, err := svg.Content(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(content, hqt.IsSameType, "")
	c.Assert(content.(string), qt.Contains, `<svg height="100" width="100">`)
}

func TestImageExif(t *testing.T) {
	c := qt.New(t)
	fs := afero.NewMemMapFs()
	spec := newTestResourceSpec(specDescriptor{fs: fs, c: c})
	image := fetchResourceForSpec(spec, c, "sunset.jpg").(images.ImageResource)

	getAndCheckExif := func(c *qt.C, image images.ImageResource) {
		x := image.Exif()
		c.Assert(x, qt.Not(qt.IsNil))

		c.Assert(x.Date.Format("2006-01-02"), qt.Equals, "2017-10-27")

		// Malaga: https://goo.gl/taazZy
		c.Assert(x.Lat, qt.Equals, float64(36.59744166666667))
		c.Assert(x.Long, qt.Equals, float64(-4.50846))

		v, found := x.Tags["LensModel"]
		c.Assert(found, qt.Equals, true)
		lensModel, ok := v.(string)
		c.Assert(ok, qt.Equals, true)
		c.Assert(lensModel, qt.Equals, "smc PENTAX-DA* 16-50mm F2.8 ED AL [IF] SDM")
		resized, _ := image.Resize("300x200")
		x2 := resized.Exif()
		c.Assert(x2, eq, x)
	}

	getAndCheckExif(c, image)
	image = fetchResourceForSpec(spec, c, "sunset.jpg").(images.ImageResource)
	// This will read from file cache.
	getAndCheckExif(c, image)
}

func TestImageColorsLuminance(t *testing.T) {
	c := qt.New(t)

	_, image := fetchSunset(c)
	c.Assert(image, qt.Not(qt.IsNil))
	colors, err := image.Colors()
	c.Assert(err, qt.IsNil)
	c.Assert(len(colors), qt.Equals, 6)
	var prevLuminance float64
	for i, color := range colors {
		luminance := color.Luminance()
		c.Assert(err, qt.IsNil)
		c.Assert(luminance > 0, qt.IsTrue)
		c.Assert(luminance, qt.Not(qt.Equals), prevLuminance, qt.Commentf("i=%d", i))
		prevLuminance = luminance
	}
}

func BenchmarkImageExif(b *testing.B) {
	getImages := func(c *qt.C, b *testing.B, fs afero.Fs) []images.ImageResource {
		spec := newTestResourceSpec(specDescriptor{fs: fs, c: c})
		imgs := make([]images.ImageResource, b.N)
		for i := 0; i < b.N; i++ {
			imgs[i] = fetchResourceForSpec(spec, c, "sunset.jpg", strconv.Itoa(i)).(images.ImageResource)
		}
		return imgs
	}

	getAndCheckExif := func(c *qt.C, image images.ImageResource) {
		x := image.Exif()
		c.Assert(x, qt.Not(qt.IsNil))
		c.Assert(x.Long, qt.Equals, float64(-4.50846))
	}

	b.Run("Cold cache", func(b *testing.B) {
		b.StopTimer()
		c := qt.New(b)
		images := getImages(c, b, afero.NewMemMapFs())

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			getAndCheckExif(c, images[i])
		}
	})

	b.Run("Cold cache, 10", func(b *testing.B) {
		b.StopTimer()
		c := qt.New(b)
		images := getImages(c, b, afero.NewMemMapFs())

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 10; j++ {
				getAndCheckExif(c, images[i])
			}
		}
	})

	b.Run("Warm cache", func(b *testing.B) {
		b.StopTimer()
		c := qt.New(b)
		fs := afero.NewMemMapFs()
		images := getImages(c, b, fs)
		for i := 0; i < b.N; i++ {
			getAndCheckExif(c, images[i])
		}

		images = getImages(c, b, fs)

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			getAndCheckExif(c, images[i])
		}
	})
}

// usesFMA indicates whether "fused multiply and add" (FMA) instruction is
// used.  The command "grep FMADD go/test/codegen/floats.go" can help keep
// the FMA-using architecture list updated.
var usesFMA = runtime.GOARCH == "s390x" ||
	runtime.GOARCH == "ppc64" ||
	runtime.GOARCH == "ppc64le" ||
	runtime.GOARCH == "arm64" ||
	runtime.GOARCH == "riscv64"

// goldenEqual compares two NRGBA images.  It is used in golden tests only.
// A small tolerance is allowed on architectures using "fused multiply and add"
// (FMA) instruction to accommodate for floating-point rounding differences
// with control golden images that were generated on amd64 architecture.
// See https://golang.org/ref/spec#Floating_point_operators
// and https://github.com/gohugoio/hugo/issues/6387 for more information.
//
// Borrowed from https://github.com/disintegration/gift/blob/a999ff8d5226e5ab14b64a94fca07c4ac3f357cf/gift_test.go#L598-L625
// Copyright (c) 2014-2019 Grigory Dryapak
// Licensed under the MIT License.
func goldenEqual(img1, img2 *image.NRGBA) bool {
	maxDiff := 0
	if usesFMA {
		maxDiff = 1
	}
	if !img1.Rect.Eq(img2.Rect) {
		return false
	}
	if len(img1.Pix) != len(img2.Pix) {
		return false
	}
	for i := 0; i < len(img1.Pix); i++ {
		diff := int(img1.Pix[i]) - int(img2.Pix[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			return false
		}
	}
	return true
}

// Issue #8729
func TestImageOperationsGoldenWebp(t *testing.T) {
	if !htesting.IsCI() {
		t.Skip("skip long running test in local mode")
	}
	if !webp.Supports() {
		t.Skip("skip webp test")
	}
	c := qt.New(t)
	c.Parallel()

	devMode := false

	testImages := []string{"fuzzy-cirlcle.png"}

	spec, workDir := newTestResourceOsFs(c)
	defer func() {
		if !devMode {
			os.Remove(workDir)
		}
	}()

	if devMode {
		fmt.Println(workDir)
	}

	for _, imageName := range testImages {
		image := fetchImageForSpec(spec, c, imageName)
		imageWebp, err := image.Resize("200x webp")
		c.Assert(err, qt.IsNil)
		c.Assert(imageWebp.Width(), qt.Equals, 200)
	}

	if devMode {
		return
	}

	dir1 := filepath.Join(workDir, "resources/_gen/images/a")
	dir2 := filepath.FromSlash("testdata/golden_webp")

	assetGoldenDirs(c, dir1, dir2)
}

func TestImageOperationsGolden(t *testing.T) {
	if !htesting.IsCI() {
		t.Skip("skip long running test in local mode")
	}
	c := qt.New(t)
	c.Parallel()

	// Note, if you're enabling this on a MacOS M1 (ARM) you need to run the test with GOARCH=amd64.
	// GOARCH=amd64 go test -count 1 -timeout 30s -run "^TestImageOperationsGolden$" ./resources -v
	// The above will print out a folder.
	// Replace testdata/golden with resources/_gen/images in that folder.
	devMode := false

	testImages := []string{"sunset.jpg", "gohugoio8.png", "gohugoio24.png"}

	spec, workDir := newTestResourceOsFs(c)
	defer func() {
		if !devMode {
			os.Remove(workDir)
		}
	}()

	if devMode {
		fmt.Println(workDir)
	}

	gopher := fetchImageForSpec(spec, c, "gopher-hero8.png")
	var err error
	gopher, err = gopher.Resize("30x")
	c.Assert(err, qt.IsNil)

	f := &images.Filters{}

	sunset := fetchImageForSpec(spec, c, "sunset.jpg")

	// Test PNGs with alpha channel.
	for _, img := range []string{"gopher-hero8.png", "gradient-circle.png"} {
		orig := fetchImageForSpec(spec, c, img)
		for _, resizeSpec := range []string{"200x #e3e615", "200x jpg #e3e615"} {
			resized, err := orig.Resize(resizeSpec)
			c.Assert(err, qt.IsNil)
			rel := resized.RelPermalink()

			c.Assert(rel, qt.Not(qt.Equals), "")

		}

		// Check the Opacity filter.
		opacity30, err := orig.Filter(f.Opacity(30))
		c.Assert(err, qt.IsNil)
		overlay, err := sunset.Filter(f.Overlay(opacity30.(images.ImageSource), 20, 20))
		c.Assert(err, qt.IsNil)
		rel := overlay.RelPermalink()
		c.Assert(rel, qt.Not(qt.Equals), "")

	}

	// A simple Gif file (no animation).
	orig := fetchImageForSpec(spec, c, "gohugoio-card.gif")
	for _, width := range []int{100, 220} {
		resized, err := orig.Resize(fmt.Sprintf("%dx", width))
		c.Assert(err, qt.IsNil)
		rel := resized.RelPermalink()
		c.Assert(rel, qt.Not(qt.Equals), "")
		c.Assert(resized.Width(), qt.Equals, width)
	}

	// Animated GIF
	orig = fetchImageForSpec(spec, c, "giphy.gif")
	for _, resizeSpec := range []string{"200x", "512x", "100x jpg"} {
		resized, err := orig.Resize(resizeSpec)
		c.Assert(err, qt.IsNil)
		rel := resized.RelPermalink()
		c.Assert(rel, qt.Not(qt.Equals), "")
	}

	for _, img := range testImages {

		orig := fetchImageForSpec(spec, c, img)
		for _, resizeSpec := range []string{"200x100", "600x", "200x r90 q50 Box"} {
			resized, err := orig.Resize(resizeSpec)
			c.Assert(err, qt.IsNil)
			rel := resized.RelPermalink()
			c.Assert(rel, qt.Not(qt.Equals), "")
		}

		for _, fillSpec := range []string{"300x200 Gaussian Smart", "100x100 Center", "300x100 TopLeft NearestNeighbor", "400x200 BottomLeft"} {
			resized, err := orig.Fill(fillSpec)
			c.Assert(err, qt.IsNil)
			rel := resized.RelPermalink()
			c.Assert(rel, qt.Not(qt.Equals), "")
		}

		for _, fitSpec := range []string{"300x200 Linear"} {
			resized, err := orig.Fit(fitSpec)
			c.Assert(err, qt.IsNil)
			rel := resized.RelPermalink()
			c.Assert(rel, qt.Not(qt.Equals), "")
		}

		filters := []gift.Filter{
			f.Grayscale(),
			f.GaussianBlur(6),
			f.Saturation(50),
			f.Sepia(100),
			f.Brightness(30),
			f.ColorBalance(10, -10, -10),
			f.Colorize(240, 50, 100),
			f.Gamma(1.5),
			f.UnsharpMask(1, 1, 0),
			f.Sigmoid(0.5, 7),
			f.Pixelate(5),
			f.Invert(),
			f.Hue(22),
			f.Contrast(32.5),
			f.Overlay(gopher.(images.ImageSource), 20, 30),
			f.Text("No options"),
			f.Text("This long text is to test line breaks. Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."),
			f.Text("Hugo rocks!", map[string]any{"x": 3, "y": 3, "size": 20, "color": "#fc03b1"}),
		}

		resized, err := orig.Fill("400x200 center")
		c.Assert(err, qt.IsNil)

		for _, filter := range filters {
			resized, err := resized.Filter(filter)
			c.Assert(err, qt.IsNil)
			rel := resized.RelPermalink()
			c.Assert(rel, qt.Not(qt.Equals), "")
		}

		resized, err = resized.Filter(filters[0:4])
		c.Assert(err, qt.IsNil)
		rel := resized.RelPermalink()
		c.Assert(rel, qt.Not(qt.Equals), "")
	}

	if devMode {
		return
	}

	dir1 := filepath.Join(workDir, "resources/_gen/images/a/")
	dir2 := filepath.FromSlash("testdata/golden")

	assetGoldenDirs(c, dir1, dir2)
}

func assetGoldenDirs(c *qt.C, dir1, dir2 string) {
	// The two dirs above should now be the same.
	dirinfos1, err := os.ReadDir(dir1)
	c.Assert(err, qt.IsNil)
	dirinfos2, err := os.ReadDir(dir2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(dirinfos1), qt.Equals, len(dirinfos2))

	for i, fi1 := range dirinfos1 {
		fi2 := dirinfos2[i]
		c.Assert(fi1.Name(), qt.Equals, fi2.Name(), qt.Commentf("i=%d", i))

		f1, err := os.Open(filepath.Join(dir1, fi1.Name()))
		c.Assert(err, qt.IsNil)
		f2, err := os.Open(filepath.Join(dir2, fi2.Name()))
		c.Assert(err, qt.IsNil)

		decodeAll := func(f *os.File) []image.Image {
			var images []image.Image

			if strings.HasSuffix(f.Name(), ".gif") {
				gif, err := gif.DecodeAll(f)
				c.Assert(err, qt.IsNil)
				images = make([]image.Image, len(gif.Image))
				for i, img := range gif.Image {
					images[i] = img
				}
			} else {
				img, _, err := image.Decode(f)
				c.Assert(err, qt.IsNil)
				images = append(images, img)
			}
			return images
		}

		imgs1 := decodeAll(f1)
		imgs2 := decodeAll(f2)
		c.Assert(len(imgs1), qt.Equals, len(imgs2))

	LOOP:
		for i, img1 := range imgs1 {
			img2 := imgs2[i]
			nrgba1 := image.NewNRGBA(img1.Bounds())
			gift.New().Draw(nrgba1, img1)
			nrgba2 := image.NewNRGBA(img2.Bounds())
			gift.New().Draw(nrgba2, img2)

			if !goldenEqual(nrgba1, nrgba2) {
				switch fi1.Name() {
				case "gohugoio8_hu7f72c00afdf7634587afaa5eff2a25b2_73538_73c19c5f80881858a85aa23cd0ca400d.png",
					"gohugoio8_hu7f72c00afdf7634587afaa5eff2a25b2_73538_ae631e5252bb5d7b92bc766ad1a89069.png",
					"gohugoio8_hu7f72c00afdf7634587afaa5eff2a25b2_73538_d1bbfa2629bffb90118cacce3fcfb924.png",
					"giphy_hu3eafc418e52414ace6236bf1d31f82e1_52213_200x0_resize_box_1.gif":
					c.Log("expectedly differs from golden due to dithering:", fi1.Name())
				default:
					c.Errorf("resulting image differs from golden: %s", fi1.Name())
					break LOOP
				}
			}
		}

		if !usesFMA {
			c.Assert(fi1, eq, fi2)

			_, err = f1.Seek(0, 0)
			c.Assert(err, qt.IsNil)
			_, err = f2.Seek(0, 0)
			c.Assert(err, qt.IsNil)

			hash1, err := helpers.MD5FromReader(f1)
			c.Assert(err, qt.IsNil)
			hash2, err := helpers.MD5FromReader(f2)
			c.Assert(err, qt.IsNil)

			c.Assert(hash1, qt.Equals, hash2)
		}

		f1.Close()
		f2.Close()
	}
}

func BenchmarkResizeParallel(b *testing.B) {
	c := qt.New(b)
	_, img := fetchSunset(c)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := rand.Intn(10) + 10
			resized, err := img.Resize(strconv.Itoa(w) + "x")
			if err != nil {
				b.Fatal(err)
			}
			_, err = resized.Resize(strconv.Itoa(w-1) + "x")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func assertWidthHeight(c *qt.C, img images.ImageResource, w, h int) {
	c.Helper()
	c.Assert(img, qt.Not(qt.IsNil))
	c.Assert(img.Width(), qt.Equals, w)
	c.Assert(img.Height(), qt.Equals, h)
}
