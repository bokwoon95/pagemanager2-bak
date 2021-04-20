package pagemanager

import (
	"html/template"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
)

type createPageData struct {
	Page
}

func (data *createPageData) Form(form *hyforms.Form) {
	url := form.Text("url", data.URL)
	pageType := form.Select("page-type", hyforms.Options{
		{Value: "", Display: "--- Select a Page Type ---", Selected: data.PageType == ""},
		{Value: PageTypeTemplate, Display: "Template", Selected: data.PageType == PageTypeTemplate},
		{Value: PageTypeContent, Display: "Content", Selected: data.PageType == PageTypeContent},
		{Value: PageTypePlugin, Display: "Plugin", Selected: data.PageType == PageTypePlugin},
		{Value: PageTypeRedirect, Display: "Redirect", Selected: data.PageType == PageTypeRedirect},
		{Value: PageTypeDisabled, Display: "Disabled", Selected: data.PageType == PageTypeDisabled},
	})
	disabled := form.Checkbox("disabled", "", data.Disabled).Set(".pointer.dib", nil)
	redirectURL := form.Text("redirect-url", data.RedirectURL)
	// TODO: change the shitty handler URL
	content := form.Textarea("content", data.Content)
	theme := form.Text("theme", data.ThemePath)
	template := form.Text("template", data.Template)

	form.Set("", hy.Attr{"method": "POST"})
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": url.ID()}, hy.Txt("URL: "))),
		hy.H("div", nil, url),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pageType.ID()}, hy.Txt("Page Type: "))),
		hy.H("div", nil, pageType),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": disabled.ID()}, hy.Txt("Disabled: "), disabled)),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": redirectURL.ID()}, hy.Txt("Redirect URL: "))),
		hy.H("div", nil, redirectURL),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": content.ID()}, hy.Txt("Content: "))),
		hy.H("div", nil, content),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": theme.ID()}, hy.Txt("Theme: "))),
		hy.H("div", nil, theme),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": template.ID()}, hy.Txt("Template: "))),
		hy.H("div", nil, template),
	)

	form.Unmarshal(func() {
		data.URL = url.Value()
		data.PageType = pageType.Value()
		data.Disabled = disabled.Checked()
		data.RedirectURL = redirectURL.Value()
		// TODO: Plugin
		data.Content = content.Value()
		data.ThemePath = theme.Value()
		data.Template = template.Value()
	})
}

func (pm *PageManager) createPage(w http.ResponseWriter, r *http.Request) {
	type templateData struct {
		Form template.HTML
	}
	data := &createPageData{}
	r.ParseForm()
	var err error
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		switch {
		case !user.Valid:
			_ = hyforms.SetCookieValue(w, cookieLoginRedirect, LocaleURL(r, r.URL.Path), nil)
			pm.RedirectToLogin(w, r)
			return
		case !user.HasPagePerms(PagePermsCreate):
			pm.Forbidden(w, r)
			return
		}
		data.URL = r.FormValue("url")
		tdata := templateData{}
		tdata.Form, err = hyforms.MarshalForm(nil, w, r, data.Form)
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		err = pm.executeTemplates(w, tdata, pagemanagerFS, "create_page.html")
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
	case "POST":
		errMsgs, ok := hyforms.UnmarshalForm(w, r, data.Form)
		if !ok {
			hyforms.Redirect(w, r, r.URL.Path, errMsgs)
			return
		}
		Redirect(w, r, URLViewPage)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
