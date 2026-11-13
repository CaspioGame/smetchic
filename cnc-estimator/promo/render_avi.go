package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

const (
	width, height = 1280, 720
	fps = 10
	framesPerScene = 5 * fps
	fadeFrames = 6
)

type aviWriter struct { b bytes.Buffer }
func (w *aviWriter) u16(v uint16) { _ = binary.Write(&w.b,binary.LittleEndian,v) }
func (w *aviWriter) u32(v uint32) { _ = binary.Write(&w.b,binary.LittleEndian,v) }
func (w *aviWriter) i32(v int32) { _ = binary.Write(&w.b,binary.LittleEndian,v) }
func (w *aviWriter) id(s string) { w.b.WriteString(s) }
func (w *aviWriter) chunk(id string,data []byte) int {
	start:=w.b.Len(); w.id(id); w.u32(uint32(len(data))); w.b.Write(data); if len(data)%2==1 {w.b.WriteByte(0)}; return start
}
func (w *aviWriter) list(kind string, body func()) {
	start:=w.b.Len(); w.id("LIST"); w.u32(0); w.id(kind); body(); end:=w.b.Len(); binary.LittleEndian.PutUint32(w.b.Bytes()[start+4:start+8],uint32(end-start-8))
}
func must(err error) { if err!=nil {panic(err)} }

func loadSlides(dir string) []*image.RGBA {
	frames:=make([]*image.RGBA,6)
	for i:=range frames {
		f,err:=os.Open(filepath.Join(dir,fmt.Sprintf("slide-%02d.png",i+1))); must(err)
		im,err:=png.Decode(f); must(err); must(f.Close())
		b:=im.Bounds(); if b.Dx()!=width||b.Dy()!=height { panic("unexpected slide dimensions") }
		frames[i]=image.NewRGBA(image.Rect(0,0,width,height)); draw.Draw(frames[i],frames[i].Bounds(),im,b.Min,draw.Src)
	}
	return frames
}

func blend(a,b *image.RGBA,alpha float64)*image.RGBA {
	out:=image.NewRGBA(image.Rect(0,0,width,height)); aa:=uint16((1-alpha)*65535); bb:=uint16(alpha*65535)
	for y:=0;y<height;y++ { for x:=0;x<width;x++ { i:=out.PixOffset(x,y); for c:=0;c<3;c++ { av:=uint16(a.Pix[i+c])*257; bv:=uint16(b.Pix[i+c])*257; out.Pix[i+c]=uint8((av*aa+bv*bb+32767)/65535) }; out.Pix[i+3]=255 } }
	return out
}

func makeGIF(slides []*image.RGBA,path string) {
	frames:=make([]*image.Paletted,len(slides)); for i,im:=range slides { pimg:=image.NewPaletted(im.Bounds(),palette.Plan9); draw.FloydSteinberg.Draw(pimg,pimg.Bounds(),im,image.Point{}); frames[i]=pimg }
	file,err:=os.Create(path); must(err); must(gif.EncodeAll(file,&gif.GIF{Image:frames,Delay:[]int{500,500,500,500,500,500},LoopCount:0,Disposal:[]byte{gif.DisposalNone,gif.DisposalNone,gif.DisposalNone,gif.DisposalNone,gif.DisposalNone,gif.DisposalNone}})); must(file.Close())
}

func makeAVI(slides []*image.RGBA,path string) {
	frames:=make([][]byte,0,len(slides)*framesPerScene); maxFrame:=0
	for scene:=0;scene<len(slides);scene++ { for n:=0;n<framesPerScene;n++ { var frame *image.RGBA; if scene<len(slides)-1&&n>=framesPerScene-fadeFrames { alpha:=float64(n-(framesPerScene-fadeFrames)+1)/float64(fadeFrames); frame=blend(slides[scene],slides[scene+1],alpha) } else { frame=slides[scene] }; var b bytes.Buffer; must(jpeg.Encode(&b,frame,&jpeg.Options{Quality:86})); f:=append([]byte(nil),b.Bytes()...); frames=append(frames,f); if len(f)>maxFrame {maxFrame=len(f)} } }
	w:=&aviWriter{}; w.id("RIFF"); w.u32(0); w.id("AVI ")
	w.list("hdrl",func(){
		var h bytes.Buffer; put32:=func(v uint32){_ = binary.Write(&h,binary.LittleEndian,v)}
		put32(1000000/fps); put32(uint32(maxFrame*fps)); put32(0); put32(0x10); put32(uint32(len(frames))); put32(0); put32(1); put32(uint32(maxFrame)); put32(width); put32(height); for i:=0;i<4;i++ {put32(0)}; w.chunk("avih",h.Bytes())
		w.list("strl",func(){
			var sh bytes.Buffer; sh.WriteString("vids"); sh.WriteString("MJPG"); putU32:=func(v uint32){_ = binary.Write(&sh,binary.LittleEndian,v)}; putU16:=func(v uint16){_ = binary.Write(&sh,binary.LittleEndian,v)}
			putU32(0);putU16(0);putU16(0);putU32(0);putU32(1);putU32(fps);putU32(0);putU32(uint32(len(frames)));putU32(uint32(maxFrame));putU32(0xffffffff);putU32(0);putU16(0);putU16(0);putU16(width);putU16(height);w.chunk("strh",sh.Bytes())
			var sf bytes.Buffer; _=binary.Write(&sf,binary.LittleEndian,uint32(40));_=binary.Write(&sf,binary.LittleEndian,int32(width));_=binary.Write(&sf,binary.LittleEndian,int32(height));_=binary.Write(&sf,binary.LittleEndian,uint16(1));_=binary.Write(&sf,binary.LittleEndian,uint16(24));sf.WriteString("MJPG");_=binary.Write(&sf,binary.LittleEndian,uint32(maxFrame));for i:=0;i<4;i++{_=binary.Write(&sf,binary.LittleEndian,int32(0))};w.chunk("strf",sf.Bytes())
		})
	})
	idx:=make([]byte,0,len(frames)*16); moviDataStart:=0; w.list("movi",func(){ moviDataStart=w.b.Len(); for _,frame:=range frames {off:=uint32(w.b.Len()-moviDataStart);w.id("00dc");w.u32(uint32(len(frame)));w.b.Write(frame);if len(frame)%2==1{w.b.WriteByte(0)}; var e bytes.Buffer;e.WriteString("00dc");_=binary.Write(&e,binary.LittleEndian,uint32(0x10));_=binary.Write(&e,binary.LittleEndian,off);_=binary.Write(&e,binary.LittleEndian,uint32(len(frame)));idx=append(idx,e.Bytes()...) } })
	w.chunk("idx1",idx);binary.LittleEndian.PutUint32(w.b.Bytes()[4:8],uint32(w.b.Len()-8));must(os.WriteFile(path,w.b.Bytes(),0644))
}

func main(){
	root:=filepath.Dir(os.Args[0]); _=root
	dir:=filepath.Join("promo","build"); out:=filepath.Join("promo","output")
	slides:=loadSlides(dir); makeAVI(slides,filepath.Join(out,"CNC-Estimator-promo-30sec.avi")); makeGIF(slides,filepath.Join(out,"CNC-Estimator-promo-30sec.gif"))
}
