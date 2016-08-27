package static

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

type _escLocalFS struct{}

var _escLocal _escLocalFS

type _escStaticFS struct{}

var _escStatic _escStaticFS

type _escDirectory struct {
	fs   http.FileSystem
	name string
}

type _escFile struct {
	compressed string
	size       int64
	modtime    int64
	local      string
	isDir      bool

	once sync.Once
	data []byte
	name string
}

func (_escLocalFS) Open(name string) (http.File, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	return os.Open(f.local)
}

func (_escStaticFS) prepare(name string) (*_escFile, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	f.once.Do(func() {
		f.name = path.Base(name)
		if f.size == 0 {
			return
		}
		var gr *gzip.Reader
		b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.compressed))
		gr, err = gzip.NewReader(b64)
		if err != nil {
			return
		}
		f.data, err = ioutil.ReadAll(gr)
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs _escStaticFS) Open(name string) (http.File, error) {
	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File()
}

func (dir _escDirectory) Open(name string) (http.File, error) {
	return dir.fs.Open(dir.name + name)
}

func (f *_escFile) File() (http.File, error) {
	type httpFile struct {
		*bytes.Reader
		*_escFile
	}
	return &httpFile{
		Reader:   bytes.NewReader(f.data),
		_escFile: f,
	}, nil
}

func (f *_escFile) Close() error {
	return nil
}

func (f *_escFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

func (f *_escFile) Stat() (os.FileInfo, error) {
	return f, nil
}

func (f *_escFile) Name() string {
	return f.name
}

func (f *_escFile) Size() int64 {
	return f.size
}

func (f *_escFile) Mode() os.FileMode {
	return 0
}

func (f *_escFile) ModTime() time.Time {
	return time.Unix(f.modtime, 0)
}

func (f *_escFile) IsDir() bool {
	return f.isDir
}

func (f *_escFile) Sys() interface{} {
	return f
}

// FS returns a http.Filesystem for the embedded assets. If useLocal is true,
// the filesystem's contents are instead used.
func FS(useLocal bool) http.FileSystem {
	if useLocal {
		return _escLocal
	}
	return _escStatic
}

// Dir returns a http.Filesystem for the embedded assets on a given prefix dir.
// If useLocal is true, the filesystem's contents are instead used.
func Dir(useLocal bool, name string) http.FileSystem {
	if useLocal {
		return _escDirectory{fs: _escLocal, name: name}
	}
	return _escDirectory{fs: _escStatic, name: name}
}

// FSByte returns the named file from the embedded assets. If useLocal is
// true, the filesystem's contents are instead used.
func FSByte(useLocal bool, name string) ([]byte, error) {
	if useLocal {
		f, err := _escLocal.Open(name)
		if err != nil {
			return nil, err
		}
		b, err := ioutil.ReadAll(f)
		f.Close()
		return b, err
	}
	f, err := _escStatic.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.data, nil
}

// FSMustByte is the same as FSByte, but panics if name is not present.
func FSMustByte(useLocal bool, name string) []byte {
	b, err := FSByte(useLocal, name)
	if err != nil {
		panic(err)
	}
	return b
}

// FSString is the string version of FSByte.
func FSString(useLocal bool, name string) (string, error) {
	b, err := FSByte(useLocal, name)
	return string(b), err
}

// FSMustString is the string version of FSMustByte.
func FSMustString(useLocal bool, name string) string {
	return string(FSMustByte(useLocal, name))
}

var _escData = map[string]*_escFile{

	"/static/css/normalize.css": {
		local:   "server/static/css/normalize.css",
		size:    7797,
		modtime: 1471461643,
		compressed: `
H4sIAAAJbogA/7RZaY/cNtL+rl9RcRDYnlfd0z2Ok7yazQcjxybI4UXsxS5gDCBKLHVzhyIFkurp9mb/
+6J46OjRTBxgnXyYtkRWFet46inq8uITUNq0TIr3uK6thcOL9WZ9Bb/DLz++hZ9Fjcoi/A474dZCXw5r
4eIyyy4vLjK4gO0a3qADjg3rpYNGKwcNa4U8gdNgmbIri0Y0a1p8tYa/GTygciBevwGHRweWBDL+r946
YI1DA9oIVI45oRXUe6Z2mMOdcHvdO+DCskoKtSNxANBbNPBe65bkX2bZ3rUS/p2BN2QVDCkmZlzD5QVs
aSnAqrUrMmFFJqyCCQVsN5vP/KqruOoOq1vh/nDlfwaX/IatPuDgkpaZnVDRvkrzk7cvPC1gcx12wg9v
f/n5JZ2vk+xEm4US5AKbAcDX/7P/prH7RhuDtYOykrq+LQflSrtgAHJotAGmTtE8lNj66Cn48Tv46vL/
139GTsnRMSFtCfQP27ctM6cyCttuLrdbksYUh++FwUYf/5z0lgk1SNtGhzPjRC0xz5gVHPMsmpBnjdjV
rCMP+9+9wTxrtHZo8myPjPu/O6P7Ls9Icp61qPo8U+yQZxbrsDMewoc0GlaAN/R6khHb9XgKoaRQuHrk
MHPnXq3h16HwDkjHYRKYFDvlQ6EbKDujdwat9af/Zm90i3nyYe4d+rpDw5JLei50ntVMHZjNs7Q5zw6C
o54fZWrttHaSIStvSAEVs0grl+ohlXyrORoFldF3Fo2Fxug2aRJqB6U3rBxqvdbKGS3telJUeKzRWtij
2O19GhKOvASOB1GjnR6wUNo9e5dk3Dyfn0tphdcZREFjGXqDX3FODoHy3V5wjuqmBOtOhDo+Tp1BOyuC
y+3G2/iD4Ahuj1A6bDvJHJb3K+Zyu83hDWuYEfk01+EvcHUVD5D05lkStGR9wI2fhbr9iBgR/U6n2hl2
gorVt1QVikOtpTYhiqx24oAgyZahnlM0vO3jvpXfV4AzTNmOGVRu6vwf286QRoOMs0pI4U5wt0cFja57
i9z7jEmrodW9RdjrA5pQNUzKIbuS8iKYlmes8Cu9Mbp3lKwT9H1L6C7xgBIstkw5UX9Ep6YE+6Osupcr
obbT2arKvHPCSbwJLtaGo1lV2jndFrDtjsC1c8iXcptUI1h01KbLSkuOxsNHSsfP/+8R1VWeWWe02o29
9i5WEol6SOHCWYOK+xp4o0bZ3tgChGNS1EvCD8wIVkmEcr8t4x7PKxSPndajilBQRugu/bsytoeShBFW
4NHZD/bCfjsxUbzHAq6wvZ429/UXX2K7DC6Pxj4qaJm5PaueAj5tmg0piWX06WazKF+oWisrrCPBZPrg
I8/RvHuWa8a29PDsZF9tPrtewPTS9lV0pe27EljTkH8JzX3jCPhaPqSpp0Tqu3NlX778jE44keBLFaDT
1pOiAgxKRoV9/VgzIoOTeKe7Alab9UsKkX9exaoJ5bLarK/Su8sL+K6tkHPkISuU++gIG6o3QJ1QRFag
ZIv9IwGraHeTuj/rYYlvEOY1Ut/5PAttJcqKAJNCcdj5jlkYrV1olmlrEfcl1/yVSBGF+KO7JuVyLOHF
UvHJF0o0HiWwuRnL3mILn2+641KhcNE0aFDVaKFCd4c4lj/J1m6P5jxz96GRrFr9flXpI+WtULsiuYSe
XfvQPPhqkXl8o5VjQo1RW66bLh5vjBDrnV46nOYcSmzLVa+Em1S+QcXRUBCXNdSayPJtxYkfYp5Z1nb3
56pWK207VmM+/ryel/J2LKnvtWk/YlP9Sek7BVK0IsyOBVSnNITlEbgnyQJawes38E86vb4jDDmFzchJ
WoJnotcWJdauzKFXkpzKqGEa3zA7ozs07gTCUjON3rtP+wNXogSuMHh9j4Z0reMgG6wX1vZYRBS1cZdu
4tiLPMGBTaNBku8jG40RaB/U9GJ9VlTe7KEGnDx9OAfonaMRSKiud3mmOxenpeAu4q5HxwwG/he7VbRm
OkqQ6fMXcfIeR2R6+GI+VAwcPdVAOZCZgFblOZOamX1WPgdhRSXxD/to6W8BPHNttGnLZDZTNYYhNIhP
LTEkjvf8KykjlNDONNwMAQWufdCiwPuaDkz2YcKZTMRRWSBzU1vGIOZndHI2DY6ioqmPijoLfNgSmuvM
2NmAkirh1UEL7ueIf2D1k3BQ9R58Xilu6M3n6836gtqfQXh29Rw4EsM8WVC+z6fpMJaLd6+fV8v5nDip
CaHS/OB0PFctRX0biKJP2xLcqUMbB8lUIWkE6W0SwPy4E9OgPlFJ1r2x2kS5qW+Ilu1wRTKjnUnN0Ejs
mRP9hZVf9I62ff0kvHhykw8lMn1L3c89uclnD21ftcI9CVNAuq9iXYfMUAQLCDKntRWsL6DTQjk0SxX2
G67s5GYvnpeSfBGNhjO9S69vZqcbngY4iPqj9Ov712dCKTTQMc4Jxch/kSHN8GmmuSh8P/aD4srvj366
/+KcOEHS9MBVwKiRYGaB4Go1xLq3/vUnou20cUx5+kvCKPn//iqkjN3j0C78Pm/RjPGG+9bZbOyeWjBY
67al/k3lxBycdA9cq6cOmHPYdm7Md7dHi/OmEU/y1IJou/AiXLVyjZaEGLQdlc/IXfLknBy0gTvB3Z5E
pcKOLqr0EcL6AYknlKe8f11ytT671UnBXuK6k3Sv91jfVvp4XgaGcaGfpFl4JF7DXHyctp1JvBfurL4X
Rx+uWZVT7ocW+NT3BeO9d8kx/op1ZtdEdaBGQ1yOpJUDIUo4Tgjib4pCzuQgHNSst2jvqw1LSc65JnJy
uCEP1zBlLKfSe59AuVzwn+rbCs2Tm6JIWOFLYmU7oVazrv7gBt27+Qbv9JS452R0kiTlCEtjw7bITL1v
BEpePngnQOkySBnDW07uMFKYl4VERH4mVC17mq4IE7ynmt71Bled0bp5vuCwYN8j+Eqe9ubPPy88Phsk
QQ8tmaD1g1I+ADWD8VCTpTLlzZJ/EiH2zo5vn1W987QkLHlODbSLKToT6MdWehzVBUjbM0vCkjnPfNce
nFXC6MPH3D6mXXiyCqoXU/XBPRxrbQLQPRTFc9Lyrb+Phwn9CymWR2YauFQ83DB7ouSUkdP+su2OYLUU
HD6tN/T/7I4IrrrjvAGtX7zEFjbrL67C3y/He4l7nxM8ry6X+P4S/51gbgqK1dCh7iQCM0j4X7N+t3eg
ewfCI88J3qPR/kE6Xur4Eneo+Fkz/WCQPftQNnzcsLXRUlbMPEDhZ4PFwzPwt74nJkLt0XZyU1nCM9Z1
UiCnOZGB6ckFlT6EXIRfX7/9rvC7BgbEFLnZsgblCSqM0MvHjy4L42U0OU1Hj96XwltiSB//Mr/V1gEN
6xT/RF2dp8U1SpmCG55MbpZrLSXrLBIIhV/X48soL/Enx/PM7f3uGbP6bwAAAP//rsjNo3UeAAA=
`,
	},

	"/static/css/skeleton.css": {
		local:   "server/static/css/skeleton.css",
		size:    11458,
		modtime: 1471461643,
		compressed: `
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
AAD//7viGWrCLAAA
`,
	},

	"/": {
		isDir: true,
		local: "server",
	},

	"/static": {
		isDir: true,
		local: "server/static",
	},

	"/static/css": {
		isDir: true,
		local: "server/static/css",
	},
}
