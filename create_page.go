package pagemanager

import (
	"html/template"
	"net/http"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
)

type createPageData struct {
	urlExists bool
	Page
}

func (data *createPageData) Form(form *hyforms.Form) {
	url := form.Text("url", data.URL)
	pageType := form.Select("page-type", hyforms.Options{
		{Value: "", Display: "--- Select a Page Type ---", Selected: data.PageType == ""},
		{Value: PageTypeTemplate, Display: "Theme Template", Selected: data.PageType == PageTypeTemplate},
		{Value: PageTypePlugin, Display: "Plugin Handler", Selected: data.PageType == PageTypePlugin},
		{Value: PageTypeContent, Display: "Content", Selected: data.PageType == PageTypeContent},
		{Value: PageTypeRedirect, Display: "Redirect", Selected: data.PageType == PageTypeRedirect},
		{Value: PageTypeDisabled, Display: "Disabled", Selected: data.PageType == PageTypeDisabled},
	})
	themePath := form.Text("theme-path", data.ThemePath)
	templateName := form.Text("template-name", data.TemplateName)
	pluginName := form.Text("plugin-name", data.PluginName)
	handlerName := form.Text("handler-name", data.HandlerName)
	content := form.Textarea("content", data.Content)
	redirectURL := form.Text("redirect-url", data.RedirectURL)
	disabled := form.Checkbox("disabled", "", data.Disabled).Set(".pointer.dib", nil)

	form.Set("", hy.Attr{"method": "POST"})
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": url.ID()}, hy.Txt("URL: "))),
		hy.H("div", nil, url),
	)
	if data.urlExists {
		form.Append("div.f6.red", nil, hy.Txt("error: url", data.URL, "already exists"))
	}
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pageType.ID()}, hy.Txt("Page Type: "))),
		hy.H("div", nil, pageType),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": themePath.ID()}, hy.Txt("Theme Path: "))),
		hy.H("div", nil, themePath),
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": templateName.ID()}, hy.Txt("Template Name: "))),
		hy.H("div", nil, templateName),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pluginName.ID()}, hy.Txt("Plugin Name: "))),
		hy.H("div", nil, pluginName),
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": handlerName.ID()}, hy.Txt("Handler Name: "))),
		hy.H("div", nil, handlerName),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": content.ID()}, hy.Txt("Content: "))),
		hy.H("div", nil, content),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": redirectURL.ID()}, hy.Txt("Redirect URL: "))),
		hy.H("div", nil, redirectURL),
	)
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": disabled.ID()}, hy.Txt("Disabled: "), disabled)),
	)

	form.Unmarshal(func() {
		data.URL = url.Value()
		data.PageType = pageType.Value()
		data.ThemePath = themePath.Value()
		data.TemplateName = templateName.Value()
		data.PluginName = pluginName.Value()
		data.HandlerName = handlerName.Value()
		data.Content = content.Value()
		data.RedirectURL = redirectURL.Value()
		data.Disabled = disabled.Checked()
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
		if data.URL != "" {
			PAGES := tables.NEW_PAGES(r.Context(), "p")
			data.urlExists, _ = sq.Exists(pm.dataDB, sq.SQLite.From(PAGES).Where(PAGES.URL.EqString(data.URL)))
		}
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
