package pagemanager

import (
	"html/template"
	"net/http"
	"net/url"

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
	const (
		TemplateGroupID = "template-group"
		PluginGroupID   = "plugin-group"
		ContentGroupID  = "content-group"
		RedirectGroupID = "redirect-group"
		DisabledGroupID = "disabled-group"
	)
	var urlValue string
	if hyforms.Validate(data.URL, hyforms.IsRelativeURL) == nil {
		urlValue = data.URL
	}
	url := form.Text("pm-url", urlValue).Set("#pm-url", nil)
	if data.PageType == "" {
		data.PageType = PageTypeTemplate
	}
	pageType := form.Select("pm-page-type", hyforms.Options{
		{Value: PageTypeTemplate, Display: "Theme Template", Selected: data.PageType == PageTypeTemplate},
		{Value: PageTypePlugin, Display: "Plugin Handler", Selected: data.PageType == PageTypePlugin},
		{Value: PageTypeContent, Display: "Content", Selected: data.PageType == PageTypeContent},
		{Value: PageTypeRedirect, Display: "Redirect", Selected: data.PageType == PageTypeRedirect},
		{Value: PageTypeDisabled, Display: "Disabled", Selected: data.PageType == PageTypeDisabled},
	}).Set("#pm-page-type", hy.Attr{"size": "5"})
	themePath := form.Text("pm-theme-path", data.ThemePath).Set("#pm-theme-path", nil)
	templateName := form.Text("pm-template-name", data.TemplateName).Set("#pm-template-name", nil)
	pluginName := form.Text("pm-plugin-name", data.PluginName).Set("#pm-plugin-name", nil)
	handlerName := form.Text("pm-handler-name", data.HandlerName).Set("#pm-handler-name", nil)
	content := form.Textarea("pm-content", data.Content).Set("#pm-content", nil)
	redirectURL := form.Text("pm-redirect-url", data.RedirectURL).Set("#pm-redirect-url", nil)
	disabled := form.Checkbox("pm-disabled", "", data.Disabled).Set("#pm-disabled.pointer.dib", nil)

	form.Set("", hy.Attr{"method": "POST"})
	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": url.ID()}, hy.Txt("URL: "))),
		hy.H("div", nil, url),
	)
	if data.urlExists {
		form.Append("div.f6.red", nil, hy.Txt("error: url", data.URL, "already exists"))
	}
	form.Append("div", nil,
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pageType.ID()}, hy.Txt("Page Type: "))),
		hy.H("div", nil, pageType),
	)
	form.Append("div[hidden]", hy.Attr{"id": TemplateGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": themePath.ID()}, hy.Txt("Theme Path: "))),
		hy.H("div", nil, themePath),
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": templateName.ID()}, hy.Txt("Template Name: "))),
		hy.H("div", nil, templateName),
	)
	form.Append("div[hidden]", hy.Attr{"id": PluginGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pluginName.ID()}, hy.Txt("Plugin Name: "))),
		hy.H("div", nil, pluginName),
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": handlerName.ID()}, hy.Txt("Handler Name: "))),
		hy.H("div", nil, handlerName),
	)
	form.Append("div[hidden]", hy.Attr{"id": ContentGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": content.ID()}, hy.Txt("Content: "))),
		hy.H("div", nil, content),
	)
	form.Append("div[hidden]", hy.Attr{"id": RedirectGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": redirectURL.ID()}, hy.Txt("Redirect URL: "))),
		hy.H("div", nil, redirectURL),
	)
	form.Append("div[hidden]", hy.Attr{"id": DisabledGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": disabled.ID()}, hy.Txt("Disabled: "), disabled)),
	)
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2.bg-white", hy.Attr{"type": "submit"}, hy.Txt("Create Page")))

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
		JS   template.HTML
	}
	data := &createPageData{}
	r.ParseForm()
	var err error
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		data.URL = r.FormValue("url")
		switch {
		case !user.Valid:
			q := url.Values{}
			if data.URL != "" {
				q.Add("url", data.URL)
			}
			_ = hyforms.SetCookieValue(w, cookieLoginRedirect, LocaleURL(r, querystringify(r.URL.Path, q)), nil)
			pm.RedirectToLogin(w, r)
			return
		case !user.HasPagePerms(PagePermsCreate):
			pm.Forbidden(w, r)
			return
		}
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
		tdata.JS, err = hy.Marshal(hy.NewSanitizer("script"), InlinedJS(w, pagemanagerFS, []string{"create_page.js"}))
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
