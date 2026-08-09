package gui

import "embed"

//go:embed locales
var localeFS embed.FS

// LocaleEnUS is the default en-US locale (all defaults).
var localeEnUS = localeDefaults()

// Locale presets loaded from embedded JSON at init time.
var (
	localeDeDE Locale
	localeArSA Locale
	localeFrFR Locale
	localeEsES Locale
	localePtBR Locale
	localeJaJP Locale
	localeZhCN Locale
	localeKoKR Locale
	localeHeIL Locale
)

func init() {
	localeDeDE = mustLoadLocale("locales/de-DE.json")
	localeArSA = mustLoadLocale("locales/ar-SA.json")
	localeFrFR = mustLoadLocale("locales/fr-FR.json")
	localeEsES = mustLoadLocale("locales/es-ES.json")
	localePtBR = mustLoadLocale("locales/pt-BR.json")
	localeJaJP = mustLoadLocale("locales/ja-JP.json")
	localeZhCN = mustLoadLocale("locales/zh-CN.json")
	localeKoKR = mustLoadLocale("locales/ko-KR.json")
	localeHeIL = mustLoadLocale("locales/he-IL.json")
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
