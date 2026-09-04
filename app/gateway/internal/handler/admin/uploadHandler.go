package admin

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"
	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"
	"luckygo/internal/ctxdata"
	"luckygo/internal/xerr"
)

var uploadExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// UploadHandler 商家图片上传（奖品图/头像/背景图）。multipart 字段名 file，≤5MB，仅图片。
// 文件落盘 {UploadDir}/{tenantId}_{随机名}{ext}，经 /api/static/uploads/:filename 公开访问。
func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := ctxdata.MustAdmin(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := r.ParseMultipartForm(6 << 20); err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.Bad("文件过大或表单不合法"))
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.Bad("缺少文件字段 file"))
			return
		}
		defer file.Close()

		buf := make([]byte, 512)
		n, err := file.Read(buf)
		if err != nil || n == 0 {
			httpx.ErrorCtx(r.Context(), w, xerr.Bad("文件为空"))
			return
		}
		ext, ok := uploadExts[http.DetectContentType(buf[:n])]
		if !ok {
			httpx.ErrorCtx(r.Context(), w, xerr.Bad("仅支持 jpg/png/gif/webp 图片"))
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.Internal())
			return
		}
		rnd := make([]byte, 8)
		if _, err := rand.Read(rnd); err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.Internal())
			return
		}
		filename := strconv.FormatUint(id.TenantID, 10) + "_" + hex.EncodeToString(rnd) + ext
		dst := filepath.Join(svcCtx.Config.UploadDir, filename)
		if err := saveUploadedFile(file, dst); err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.Internal())
			return
		}
		httpx.OkJsonCtx(r.Context(), w, &types.UploadResp{Url: "/api/static/uploads/" + filename})
	}
}

// StaticUploadHandler 公开读取上传文件（单段文件名，拒绝路径穿越）。
func StaticUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Filename string `path:"filename"`
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		name := req.Filename
		if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
			httpx.ErrorCtx(r.Context(), w, xerr.NotFound("文件不存在"))
			return
		}
		http.ServeFile(w, r, filepath.Join(svcCtx.Config.UploadDir, name))
	}
}

func saveUploadedFile(src multipart.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}
