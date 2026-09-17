package gui

import "embed"

//go:embed locales
var localeFS embed.FS

// LocaleEnUS is the default en-US locale (all defaults).
// exportaudit:keep — documented public API (widget_locale.md).
var LocaleEnUS = localeDefaults()

// Locale presets loaded from embedded JSON at init time.
// Each is registered in the locale registry; look them up with
// LocaleGet or switch with Window.SetLocaleID.
// exportaudit:keep — documented public API (widget_locale.md).
var (
	LocaleDeDE Locale
	LocaleArSA Locale
	LocaleFrFR Locale
	LocaleEsES Locale
	LocalePtBR Locale
	LocaleJaJP Locale
	LocaleZhCN Locale
	LocaleKoKR Locale
	LocaleHeIL Locale
)

func init() {
	LocaleDeDE = mustLoadLocale("locales/de-DE.json")
	LocaleArSA = mustLoadLocale("locales/ar-SA.json")
	LocaleFrFR = mustLoadLocale("locales/fr-FR.json")
	LocaleEsES = mustLoadLocale("locales/es-ES.json")
	LocalePtBR = mustLoadLocale("locales/pt-BR.json")
	LocaleJaJP = mustLoadLocale("locales/ja-JP.json")
	LocaleZhCN = mustLoadLocale("locales/zh-CN.json")
	LocaleKoKR = mustLoadLocale("locales/ko-KR.json")
	LocaleHeIL = mustLoadLocale("locales/he-IL.json")
}

func mustLoadLocale(path string) Locale {
	data, err := localeFS.ReadFile(path)
	if err != nil {
		panic("gui: locale " + path + ": " + err.Error())
	}
	l, err := LocaleParse(string(data))
	if err != nil {
		panic("gui: locale " + path + ": " + err.Error())
	}
	return l
}
