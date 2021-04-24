package pagemanager

import (
	"html/template"
	"net/http"
	"sort"

	"github.com/bokwoon95/pagemanager/erro"
	"github.com/bokwoon95/pagemanager/hy"
	"github.com/bokwoon95/pagemanager/hyforms"
	"github.com/bokwoon95/pagemanager/sq"
	"github.com/bokwoon95/pagemanager/tables"
	"github.com/bokwoon95/pagemanager/tpl"
)

type createPageData struct {
	w http.ResponseWriter `json:"-"`
	r *http.Request       `json:"-"`
	Page
	URLExists bool
	Themes    []string
	Templates [][]string
}

func (data *createPageData) Form() (template.HTML, error) {
	return hyforms.MarshalForm(nil, data.w, data.r, data.formCallback)
}

func (data *createPageData) JS() (template.HTML, error) {
	return hy.Marshal(hy.UnsafeSanitizer(), InlinedJS(data.w, pagemanagerFS, []string{"create_page.js"}))
}

func (data *createPageData) formCallback(form *hyforms.Form) {
	const (
		TemplateGroupID = "template-group"
		PluginGroupID   = "plugin-group"
		ContentGroupID  = "content-group"
		RedirectGroupID = "redirect-group"
		DisabledGroupID = "disabled-group"
	)
	form.Set("#pm-create-page", hy.Attr{"method": "POST"})
	var urlValue string
	if hyforms.Validate(data.URL, hyforms.IsRelativeURL) == nil {
		urlValue = data.URL
	}
	pageURL := form.Text("pm-url", urlValue).Set("#pm-url", nil)
	if data.PageType == "" {
		data.PageType = PageTypeTemplate
	}
	pageType := form.Select("pm-page-type", hyforms.Options{
		{Value: PageTypeTemplate, Display: "Theme Template", Selected: data.PageType == PageTypeTemplate},
		{Value: PageTypePlugin, Display: "Plugin Handler", Selected: data.PageType == PageTypePlugin},
		{Value: PageTypeContent, Display: "Content", Selected: data.PageType == PageTypeContent},
		{Value: PageTypeRedirect, Display: "Redirect", Selected: data.PageType == PageTypeRedirect},
		{Value: PageTypeDisabled, Display: "Disabled", Selected: data.PageType == PageTypeDisabled},
	}).Set("#pm-page-type.pointer", hy.Attr{"size": "5"})
	var themePathOptions hyforms.Options
	for i, themeName := range data.Themes {
		themePathOptions.Append(hyforms.Option{Value: themeName, Display: themeName, Selected: i == 0})
	}
	themePath := func() *hyforms.SelectInput {
		var opts hyforms.Options
		for i, themeName := range data.Themes {
			opts.Append(hyforms.Option{Value: themeName, Display: themeName, Selected: i == 0})
		}
		return form.Select("pm-theme-path", opts).Set("#pm-theme-path.pointer", hy.Attr{"size": "5"})
	}()
	templateNames := func() hy.Elements {
		const prefix = "pm-templatefor-"
		var els hy.Elements
		for i, themeName := range data.Themes {
			name := prefix + themeName
			if len(data.Templates[i]) > 0 {
				var opts hyforms.Options
				for j, templateName := range data.Templates[i] {
					opts.Append(hyforms.Option{Value: templateName, Display: templateName, Selected: j == 0})
				}
				els.Append("div", hy.Attr{"id": name}, form.Select(name, opts).Set(".pointer", hy.Attr{"size": "5"}))
			} else {
				els.Append("div", hy.Attr{"id": name}, form.Select(name, hyforms.Options{{Display: "<empty>"}}))
			}
		}
		return els
	}()
	pluginName := form.Text("pm-plugin-name", data.PluginName).Set("#pm-plugin-name", nil)
	handlerName := form.Text("pm-handler-name", data.HandlerName).Set("#pm-handler-name", nil)
	content := form.Textarea("pm-content", data.Content).Set("#pm-content", nil)
	redirectURL := form.Text("pm-redirect-url", data.RedirectURL).Set("#pm-redirect-url", nil)
	disabled := form.Checkbox("pm-disabled", "", data.Disabled).Set("#pm-disabled.pointer.dib", nil)

	form.AppendElements(
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pageURL.ID()}, hy.Txt("URL: "))),
		hy.H("div", nil, pageURL),
	)
	if data.URLExists {
		form.Append("div.f6.red", nil, hy.Txt("error: url", data.URL, "already exists"))
	} else if data.URL == "/" {
		form.Append("div.f6.gray", nil, hy.Txt(`Note: "/" refers to your home page.`))
	}
	form.Append("div", nil,
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pageType.ID()}, hy.Txt("Page Type: "))),
		hy.H("div", nil, pageType),
	)
	form.Append("div", hy.Attr{"id": TemplateGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": themePath.ID()}, hy.Txt("Theme Path: "))),
		hy.H("div", nil, themePath),
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{}, hy.Txt("Template Name: "))),
		templateNames,
	)
	form.Append("div", hy.Attr{"id": PluginGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": pluginName.ID()}, hy.Txt("Plugin Name: "))),
		hy.H("div", nil, pluginName),
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": handlerName.ID()}, hy.Txt("Handler Name: "))),
		hy.H("div", nil, handlerName),
	)
	form.Append("div", hy.Attr{"id": ContentGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": content.ID()}, hy.Txt("Content: "))),
		hy.H("div", nil, content),
	)
	form.Append("div", hy.Attr{"id": RedirectGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": redirectURL.ID()}, hy.Txt("Redirect URL: "))),
		hy.H("div", nil, redirectURL),
	)
	form.Append("div", hy.Attr{"id": DisabledGroupID},
		hy.H("div.mt3.mb1", nil, hy.H("label.pointer", hy.Attr{"for": disabled.ID()}, hy.Txt("Disabled: "), disabled)),
	)
	form.Append("div.mt3", nil, hy.H("button.pointer.pa2.bg-white", hy.Attr{"type": "submit"}, hy.Txt("Create Page")))

	form.Unmarshal(func() {
		data.Valid = true
		data.URL = pageURL.Value()
		data.PageType = pageType.Value()
		data.ThemePath = themePath.Value()
		data.TemplateName = form.Request().FormValue("pm-templatefor-" + data.ThemePath)
		data.PluginName = pluginName.Value()
		data.HandlerName = handlerName.Value()
		data.Content = content.Value()
		data.RedirectURL = redirectURL.Value()
		data.Disabled = disabled.Checked()
	})
}

func (data *createPageData) processThemes(pmThemes map[string]theme) {
	data.Themes = make([]string, len(pmThemes))
	data.Templates = make([][]string, len(pmThemes))
	i := 0
	for themeName := range pmThemes {
		data.Themes[i] = themeName
		i++
	}
	sort.Strings(data.Themes)
	for i, themeName := range data.Themes {
		theme := pmThemes[themeName]
		templates := make([]string, len(theme.themeTemplates))
		j := 0
		for templateName := range theme.themeTemplates {
			templates[j] = templateName
			j++
		}
		sort.Strings(templates)
		data.Templates[i] = templates
	}
}

func (pm *PageManager) createPage(w http.ResponseWriter, r *http.Request) {
	data := &createPageData{w: w, r: r}
	r.ParseForm()
	switch r.Method {
	case "GET":
		user := pm.getUser(w, r)
		switch {
		case !user.Valid:
			pm.RedirectToLogin(w, r)
			return
		case !user.HasPagePerms(PagePermsCreate):
			pm.Forbidden(w, r)
			return
		}
		data.URL = r.FormValue("url")
		if data.URL != "" {
			PAGES := tables.NEW_PAGES(r.Context(), "p")
			data.URLExists, _ = sq.Exists(pm.dataDB, sq.SQLite.From(PAGES).Where(PAGES.URL.EqString(data.URL)))
		}
		err := pm.refreshThemes()
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
		data.processThemes(pm.themes)
		err = pm.tpl.Render(w, r, data, tpl.Files("create_page.html"))
		if err != nil {
			pm.InternalServerError(w, r, erro.Wrap(err))
			return
		}
	case "POST":
		errMsgs, ok := hyforms.UnmarshalForm(w, r, data.formCallback)
		if !ok {
			hyforms.Redirect(w, r, LocaleURL(r, r.URL.Path), errMsgs)
			return
		}
		Redirect(w, r, r.URL.Path)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
