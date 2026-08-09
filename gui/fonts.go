package gui

import (
	"bytes"
	_ "embed"
	"log"
	"path/filepath"
	"slices"
)

// FontVariants holds paths of font files used by the GUI.
type fontVariants struct {
	normal string
	Bold   string
	Italic string
	mono   string
}

// Font name constants.
const (
	baseFontName = defaultFontFamily
	IconFontName = "feathericon"
)

// IconFontData is the embedded Feather icon TTF font data.
//
//go:embed assets/feathericon.ttf
var IconFontData []byte

// AppFontPaths lists application font files to register with the
// text system at backend init time. Append paths via RegisterAppFont
// before calling backend.RunApp / backend.Run. Paths are loaded
// process-globally so any window can resolve them by Family name.
//
// Use this for fonts not exposed through fontconfig / system font
// catalogs — for example, fonts bundled inside .app resource
// directories on macOS.
var appFontPaths []string

// RegisterAppFont appends path to AppFontPaths if not already
// present. Safe to call multiple times with the same path.
func RegisterAppFont(path string) {
	if slices.Contains(appFontPaths, path) {
		return
	}
	appFontPaths = append(appFontPaths, path)
}

// AppFontData lists in-memory application fonts (TTF/OTF) to register
// with the text system at backend init time. Append via
// RegisterAppFontBytes before calling backend.RunApp / backend.Run.
//
// Use this for fonts embedded in the executable with go:embed — the
// text system persists the bytes to its own temp file, so the caller
// does not have to manage one.
//
// Consumed by the GL, Metal, iOS and Android backends, matching where
// AppFontPaths is consumed. Only the web backend ignores both lists; on
// web use the browser FontFace API instead.
//
// To retarget the themed icon styles at a registered font, set that
// font's family name in ThemeCfg.IconFontFamily.
var appFontData [][]byte

// RegisterAppFontBytes appends data to AppFontData if not already
// present. Empty data is ignored. Safe to call multiple times with the
// same font.
//
// data is retained by reference, not copied — the caller must not
// mutate it afterwards. go:embed data satisfies this by construction.
func registerAppFontBytes(data []byte) {
	if len(data) == 0 {
		return
	}
	// slices.Contains needs a comparable element; []byte is not.
	if slices.ContainsFunc(appFontData, func(d []byte) bool {
		return bytes.Equal(d, data)
	}) {
		return
	}
	appFontData = append(appFontData, data)
}

// FontRegistrar is the font-loading subset of glyph.TextSystem that
// LoadAppFonts needs. Backends pass their *glyph.TextSystem; tests pass
// a fake.
// exportaudit:keep — reachable from an exported signature
type FontRegistrar interface {
	AddFontFile(path string) error
	AddFontBytes(data []byte) error
}

// LoadAppFonts registers every font in AppFontPaths and AppFontData
// with ts, naming the calling backend ("gl", "metal", "ios", "android")
// in log lines.
// Call once per text system during backend init, after the bundled icon
// font is loaded.
//
// A font that fails to load is logged and skipped: one unreadable font
// must not stop the remaining ones — or the window itself — from coming
// up. A nil ts is a no-op.
func LoadAppFonts(ts FontRegistrar, backend string) {
	if ts == nil {
		return
	}
	for _, p := range appFontPaths {
		if err := ts.AddFontFile(p); err != nil {
			log.Printf("%s: load app font %q: %v",
				backend, filepath.Base(p), err)
		}
	}
	// AddFontBytes owns the temp file it writes and removes it in Free,
	// so there is nothing to track here. Byte slices have no name to
	// report, so identify a failure by index and size.
	for i, d := range appFontData {
		if err := ts.AddFontBytes(d); err != nil {
			log.Printf("%s: load app font bytes #%d (%d bytes): %v",
				backend, i, len(d), err)
		}
	}
}

// Icon constants — Feather icon font Unicode mappings.
const (
	iconArrowDown              = "\uf100"
	iconArrowLeft              = "\uf101"
	iconArrowRight             = "\uf102"
	iconArrowUp                = "\uf103"
	iconArtboard               = "\uf104"
	iconBar                    = "\uf105"
	iconBarChart               = "\uf106"
	iconBeer                   = "\uf107"
	IconBell                   = "\uf108"
	IconBook                   = "\uf109"
	iconBrowser                = "\uf10b"
	iconBrush                  = "\uf10c"
	iconBug                    = "\uf10d"
	iconBuilding               = "\uf10e"
	iconCalendar               = "\uf10f"
	iconCamera                 = "\uf110"
	iconCheck                  = "\uf111"
	iconClock                  = "\uf113"
	iconClose                  = "\uf114"
	iconCloud                  = "\uf115"
	iconCocktail               = "\uf116"
	iconCode                   = "\uf117"
	iconColumns                = "\uf118"
	iconComment                = "\uf119"
	iconCommenting             = "\uf11a"
	iconComments               = "\uf11b"
	iconDesktop                = "\uf11d"
	iconDiamond                = "\uf11e"
	iconDisabled               = "\uf11f"
	IconDownload               = "\uf120"
	iconDropDown               = "\uf121"
	iconDropLeft               = "\uf122"
	iconDropRight              = "\uf123"
	iconDropUp                 = "\uf124"
	iconElipsisH               = "\uf125"
	iconElipsisV               = "\uf126"
	iconEye                    = "\uf127"
	iconFeed                   = "\uf128"
	IconFlag                   = "\uf129"
	IconFolder                 = "\uf12a"
	iconFork                   = "\uf12b"
	iconGlobe                  = "\uf12c"
	iconHash                   = "\uf12d"
	IconHeart                  = "\uf12e"
	iconHome                   = "\uf12f"
	iconInfo                   = "\uf130"
	iconKey                    = "\uf131"
	IconKeyboard               = "\uf132"
	iconLaptop                 = "\uf133"
	iconLayout                 = "\uf134"
	iconLineChart              = "\uf135"
	iconLink                   = "\uf136"
	iconLinkExternal           = "\uf137"
	iconLocation               = "\uf138"
	iconLock                   = "\uf139"
	iconLogin                  = "\uf13a"
	iconLogout                 = "\uf13b"
	iconMail                   = "\uf13c"
	iconMedal                  = "\uf13d"
	iconMegaphone              = "\uf13e"
	iconMinus                  = "\uf140"
	iconMobile                 = "\uf141"
	iconMouse                  = "\uf142"
	iconPencil                 = "\uf144"
	iconPhone                  = "\uf145"
	iconPieChart               = "\uf146"
	iconPizza                  = "\uf147"
	IconPlus                   = "\uf148"
	iconPrototype              = "\uf149"
	iconQuestion               = "\uf14a"
	iconQuoteLeft              = "\uf14b"
	iconQuoteRight             = "\uf14c"
	IconRocket                 = "\uf14d"
	IconSearch                 = "\uf14e"
	iconShare                  = "\uf14f"
	iconSitemap                = "\uf150"
	IconStar                   = "\uf151"
	iconTablet                 = "\uf152"
	iconTag                    = "\uf153"
	iconTerminal               = "\uf154"
	iconTicket                 = "\uf155"
	iconTiled                  = "\uf156"
	IconTrash                  = "\uf157"
	iconTrophy                 = "\uf158"
	iconUpload                 = "\uf159"
	IconUser                   = "\uf15a"
	iconUserPlus               = "\uf15b"
	iconUsers                  = "\uf15c"
	iconVector                 = "\uf15d"
	iconVideo                  = "\uf15e"
	iconWarning                = "\uf15f"
	iconWineGlass              = "\uf161"
	iconWrench                 = "\uf162"
	iconBirthdayCake           = "\uf163"
	iconMention                = "\uf164"
	iconPalette                = "\uf165"
	iconCoffee                 = "\uf166"
	iconHeartO                 = "\uf167"
	IconStarO                  = "\uf168"
	iconUnlock                 = "\uf169"
	iconSearchMinus            = "\uf16a"
	iconSearchPlus             = "\uf16b"
	iconUserMinus              = "\uf16c"
	iconMapIcon                = "\uf16d"
	IconExport                 = "\uf16e"
	iconImport                 = "\uf16f"
	iconBookmark               = "\uf170"
	IconPrint                  = "\uf171"
	iconShield                 = "\uf172"
	iconFilter                 = "\uf173"
	iconFeather                = "\uf174"
	iconMusic                  = "\uf175"
	iconFolderOpen             = "\uf176"
	iconMagic                  = "\uf177"
	iconPaperPlane             = "\uf178"
	iconBold                   = "\uf179"
	iconItalic                 = "\uf17a"
	iconTextSize               = "\uf17b"
	iconListBullet             = "\uf17c"
	iconListOrder              = "\uf17d"
	IconListTask               = "\uf17e"
	iconEdit                   = "\uf17f"
	iconBackward               = "\uf180"
	iconCompress               = "\uf181"
	iconEject                  = "\uf182"
	iconExpand                 = "\uf183"
	iconFastBackward           = "\uf184"
	iconFastForward            = "\uf185"
	iconForward                = "\uf186"
	iconPause                  = "\uf187"
	IconPlay                   = "\uf188"
	iconRandom                 = "\uf189"
	IconStop                   = "\uf18b"
	iconLayer                  = "\uf18c"
	iconHeadphone              = "\uf18e"
	iconPlug                   = "\uf18f"
	iconUsb                    = "\uf190"
	IconGamepad                = "\uf191"
	iconLoop                   = "\uf192"
	IconSync                   = "\uf194"
	iconAlignCenter            = "\uf195"
	iconAlignLeft              = "\uf196"
	iconAlignRight             = "\uf197"
	iconAppMenu                = "\uf198"
	iconAudioPlayer            = "\uf199"
	iconCheckCircle            = "\uf19a"
	iconCheckCircleO           = "\uf19b"
	iconCheckVerified          = "\uf19c"
	iconCutlery                = "\uf19d"
	iconDeleteLink             = "\uf19e"
	iconDocument               = "\uf19f"
	iconEqualizer              = "\uf1a0"
	iconFileExcel              = "\uf1a2"
	iconFilePowerpoint         = "\uf1a3"
	iconFileWord               = "\uf1a4"
	iconGear                   = "\uf1a5"
	iconInsertLink             = "\uf1a6"
	iconKitchenCooker          = "\uf1a7"
	iconMoney                  = "\uf1a8"
	iconPicture                = "\uf1a9"
	iconPot                    = "\uf1aa"
	iconSpeaker                = "\uf1ab"
	iconTable                  = "\uf1ac"
	iconTimeline               = "\uf1ad"
	iconUnderline              = "\uf1ae"
	iconWatch                  = "\uf1af"
	iconWatchAlt               = "\uf1b0"
	iconFile                   = "\uf1b1"
	iconFileAudio              = "\uf1b2"
	iconFileImage              = "\uf1b3"
	iconFileMovie              = "\uf1b4"
	iconFileZip                = "\uf1b5"
	iconAngry                  = "\uf1b6"
	iconCry                    = "\uf1b7"
	iconDisappointed           = "\uf1b8"
	iconFrowing                = "\uf1b9"
	iconOpenMouth              = "\uf1ba"
	iconRage                   = "\uf1bb"
	IconSmile                  = "\uf1bc"
	IconSmileAlt               = "\uf1bd"
	iconTired                  = "\uf1be"
	iconAlignBottom            = "\uf1bf"
	iconAlignTop               = "\uf1c0"
	iconAlignVertically        = "\uf1c1"
	iconCrop                   = "\uf1c2"
	iconDifference             = "\uf1c3"
	iconDistributeVertically   = "\uf1c5"
	iconEraser                 = "\uf1c6"
	iconIntersect              = "\uf1c7"
	iconMask                   = "\uf1c8"
	iconScale                  = "\uf1c9"
	iconSubtract               = "\uf1ca"
	iconTextAlignCenter        = "\uf1cb"
	iconTextAlignLeft          = "\uf1cc"
	iconTextAlignRight         = "\uf1cd"
	iconUnion                  = "\uf1ce"
	iconDistributeHorizontally = "\uf1cf"
	iconStepBackward           = "\uf1d0"
	iconStepForward            = "\uf1d1"
	iconCommentO               = "\uf1d2"
	iconCodepen                = "\uf1d3"
	iconFacebook               = "\uf1d4"
	iconGit                    = "\uf1d5"
	iconGithub                 = "\uf1d6"
	IconGithubAlt              = "\uf1d7"
	iconGoogle                 = "\uf1d8"
	iconGooglePlus             = "\uf1d9"
	iconInstagram              = "\uf1da"
	iconPinterest              = "\uf1db"
	iconPocket                 = "\uf1dc"
	IconTwitter                = "\uf1dd"
	iconWordpress              = "\uf1de"
	iconWordpressAlt           = "\uf1df"
	iconYoutube                = "\uf1e0"
	iconMessanger              = "\uf1e1"
	iconActivity               = "\uf1e2"
	iconBolt                   = "\uf1e3"
	iconPictureSquare          = "\uf1e4"
	iconTextAlignJustify       = "\uf1e5"
	iconAddCart                = "\uf1e6"
	IconCage                   = "\uf1e7"
	iconCart                   = "\uf1e8"
	iconCreditCard             = "\uf1e9"
	iconGift                   = "\uf1ea"
	iconRemoveCart             = "\uf1eb"
	iconShoppingBag            = "\uf1ec"
	iconTruck                  = "\uf1ed"
	iconWallet                 = "\uf1ee"
	IconMoon                   = "\uf1ef"
	IconSunnyO                 = "\uf1f0"
	iconSunrise                = "\uf1f1"
	iconUmbrella               = "\uf1f2"
	IconTarget                 = "\uf1f3"
	iconSmilePlus              = "\uf1f5"
	IconSmileHeart             = "\uf1f6"
	iconBeginner               = "\uf1f7"
	iconTrain                  = "\uf1f8"
	iconDonut                  = "\uf1f9"
	iconRiceCracker            = "\uf1fa"
	iconApron                  = "\uf1fb"
	iconOctpus                 = "\uf1fc"
	iconSquid                  = "\uf1fd"
	iconBus                    = "\uf1fe"
	iconCar                    = "\uf1ff"
	iconNoticeActive           = "\uf200"
	iconNoticeOff              = "\uf201"
	iconNoticeOn               = "\uf202"
	iconNoticePush             = "\uf203"
	iconTaxi                   = "\uf204"
	iconVr                     = "\uf205"
	iconBread                  = "\uf206"
	iconFryingPan              = "\uf207"
	iconMitarashiDango         = "\uf208"
	iconTumblerGlass           = "\uf209"
	iconYakiDango              = "\uf20a"
)

// IconLookup maps V-style snake_case icon names to Unicode values.
var IconLookup = map[string]string{
	"icon_arrow_down":              iconArrowDown,
	"icon_arrow_left":              iconArrowLeft,
	"icon_arrow_right":             iconArrowRight,
	"icon_arrow_up":                iconArrowUp,
	"icon_artboard":                iconArtboard,
	"icon_bar":                     iconBar,
	"icon_bar_chart":               iconBarChart,
	"icon_beer":                    iconBeer,
	"icon_bell":                    IconBell,
	"icon_book":                    IconBook,
	"icon_browser":                 iconBrowser,
	"icon_brush":                   iconBrush,
	"icon_bug":                     iconBug,
	"icon_building":                iconBuilding,
	"icon_calendar":                iconCalendar,
	"icon_camera":                  iconCamera,
	"icon_check":                   iconCheck,
	"icon_clock":                   iconClock,
	"icon_close":                   iconClose,
	"icon_cloud":                   iconCloud,
	"icon_cocktail":                iconCocktail,
	"icon_code":                    iconCode,
	"icon_columns":                 iconColumns,
	"icon_comment":                 iconComment,
	"icon_commenting":              iconCommenting,
	"icon_comments":                iconComments,
	"icon_desktop":                 iconDesktop,
	"icon_diamond":                 iconDiamond,
	"icon_disabled":                iconDisabled,
	"icon_download":                IconDownload,
	"icon_drop_down":               iconDropDown,
	"icon_drop_left":               iconDropLeft,
	"icon_drop_right":              iconDropRight,
	"icon_drop_up":                 iconDropUp,
	"icon_elipsis_h":               iconElipsisH,
	"icon_elipsis_v":               iconElipsisV,
	"icon_eye":                     iconEye,
	"icon_feed":                    iconFeed,
	"icon_flag":                    IconFlag,
	"icon_folder":                  IconFolder,
	"icon_fork":                    iconFork,
	"icon_globe":                   iconGlobe,
	"icon_hash":                    iconHash,
	"icon_heart":                   IconHeart,
	"icon_home":                    iconHome,
	"icon_info":                    iconInfo,
	"icon_key":                     iconKey,
	"icon_keyboard":                IconKeyboard,
	"icon_laptop":                  iconLaptop,
	"icon_layout":                  iconLayout,
	"icon_line_chart":              iconLineChart,
	"icon_link":                    iconLink,
	"icon_link_external":           iconLinkExternal,
	"icon_location":                iconLocation,
	"icon_lock":                    iconLock,
	"icon_login":                   iconLogin,
	"icon_logout":                  iconLogout,
	"icon_mail":                    iconMail,
	"icon_medal":                   iconMedal,
	"icon_megaphone":               iconMegaphone,
	"icon_minus":                   iconMinus,
	"icon_mobile":                  iconMobile,
	"icon_mouse":                   iconMouse,
	"icon_pencil":                  iconPencil,
	"icon_phone":                   iconPhone,
	"icon_pie_chart":               iconPieChart,
	"icon_pizza":                   iconPizza,
	"icon_plus":                    IconPlus,
	"icon_prototype":               iconPrototype,
	"icon_question":                iconQuestion,
	"icon_quote_left":              iconQuoteLeft,
	"icon_quote_right":             iconQuoteRight,
	"icon_rocket":                  IconRocket,
	"icon_search":                  IconSearch,
	"icon_share":                   iconShare,
	"icon_sitemap":                 iconSitemap,
	"icon_star":                    IconStar,
	"icon_tablet":                  iconTablet,
	"icon_tag":                     iconTag,
	"icon_terminal":                iconTerminal,
	"icon_ticket":                  iconTicket,
	"icon_tiled":                   iconTiled,
	"icon_trash":                   IconTrash,
	"icon_trophy":                  iconTrophy,
	"icon_upload":                  iconUpload,
	"icon_user":                    IconUser,
	"icon_user_plus":               iconUserPlus,
	"icon_users":                   iconUsers,
	"icon_vector":                  iconVector,
	"icon_video":                   iconVideo,
	"icon_warning":                 iconWarning,
	"icon_wine_glass":              iconWineGlass,
	"icon_wrench":                  iconWrench,
	"icon_birthday_cake":           iconBirthdayCake,
	"icon_mention":                 iconMention,
	"icon_palette":                 iconPalette,
	"icon_coffee":                  iconCoffee,
	"icon_heart_o":                 iconHeartO,
	"icon_star_o":                  IconStarO,
	"icon_unlock":                  iconUnlock,
	"icon_search_minus":            iconSearchMinus,
	"icon_search_plus":             iconSearchPlus,
	"icon_user_minus":              iconUserMinus,
	"icon_map":                     iconMapIcon,
	"icon_export":                  IconExport,
	"icon_import":                  iconImport,
	"icon_bookmark":                iconBookmark,
	"icon_print":                   IconPrint,
	"icon_shield":                  iconShield,
	"icon_filter":                  iconFilter,
	"icon_feather":                 iconFeather,
	"icon_music":                   iconMusic,
	"icon_folder_open":             iconFolderOpen,
	"icon_magic":                   iconMagic,
	"icon_paper_plane":             iconPaperPlane,
	"icon_bold":                    iconBold,
	"icon_italic":                  iconItalic,
	"icon_text_size":               iconTextSize,
	"icon_list_bullet":             iconListBullet,
	"icon_list_order":              iconListOrder,
	"icon_list_task":               IconListTask,
	"icon_edit":                    iconEdit,
	"icon_backward":                iconBackward,
	"icon_compress":                iconCompress,
	"icon_eject":                   iconEject,
	"icon_expand":                  iconExpand,
	"icon_fast_backward":           iconFastBackward,
	"icon_fast_forward":            iconFastForward,
	"icon_forward":                 iconForward,
	"icon_pause":                   iconPause,
	"icon_play":                    IconPlay,
	"icon_random":                  iconRandom,
	"icon_stop":                    IconStop,
	"icon_layer":                   iconLayer,
	"icon_headphone":               iconHeadphone,
	"icon_plug":                    iconPlug,
	"icon_usb":                     iconUsb,
	"icon_gamepad":                 IconGamepad,
	"icon_loop":                    iconLoop,
	"icon_sync":                    IconSync,
	"icon_align_center":            iconAlignCenter,
	"icon_align_left":              iconAlignLeft,
	"icon_align_right":             iconAlignRight,
	"icon_app_menu":                iconAppMenu,
	"icon_audio_player":            iconAudioPlayer,
	"icon_check_circle":            iconCheckCircle,
	"icon_check_circle_o":          iconCheckCircleO,
	"icon_check_verified":          iconCheckVerified,
	"icon_cutlery":                 iconCutlery,
	"icon_delete_link":             iconDeleteLink,
	"icon_document":                iconDocument,
	"icon_equalizer":               iconEqualizer,
	"icon_file_excel":              iconFileExcel,
	"icon_file_powerpoint":         iconFilePowerpoint,
	"icon_file_word":               iconFileWord,
	"icon_gear":                    iconGear,
	"icon_insert_link":             iconInsertLink,
	"icon_kitchen_cooker":          iconKitchenCooker,
	"icon_money":                   iconMoney,
	"icon_picture":                 iconPicture,
	"icon_pot":                     iconPot,
	"icon_speaker":                 iconSpeaker,
	"icon_table":                   iconTable,
	"icon_timeline":                iconTimeline,
	"icon_underline":               iconUnderline,
	"icon_watch":                   iconWatch,
	"icon_watch_alt":               iconWatchAlt,
	"icon_file":                    iconFile,
	"icon_file_audio":              iconFileAudio,
	"icon_file_image":              iconFileImage,
	"icon_file_movie":              iconFileMovie,
	"icon_file_zip":                iconFileZip,
	"icon_angry":                   iconAngry,
	"icon_cry":                     iconCry,
	"icon_disappointed":            iconDisappointed,
	"icon_frowing":                 iconFrowing,
	"icon_open_mouth":              iconOpenMouth,
	"icon_rage":                    iconRage,
	"icon_smile":                   IconSmile,
	"icon_smile_alt":               IconSmileAlt,
	"icon_tired":                   iconTired,
	"icon_align_bottom":            iconAlignBottom,
	"icon_align_top":               iconAlignTop,
	"icon_align_vertically":        iconAlignVertically,
	"icon_crop":                    iconCrop,
	"icon_difference":              iconDifference,
	"icon_distribute_vertically":   iconDistributeVertically,
	"icon_eraser":                  iconEraser,
	"icon_intersect":               iconIntersect,
	"icon_mask":                    iconMask,
	"icon_scale":                   iconScale,
	"icon_subtract":                iconSubtract,
	"icon_text_align_center":       iconTextAlignCenter,
	"icon_text_align_left":         iconTextAlignLeft,
	"icon_text_align_right":        iconTextAlignRight,
	"icon_union":                   iconUnion,
	"icon_distribute_horizontally": iconDistributeHorizontally,
	"icon_step_backward":           iconStepBackward,
	"icon_step_forward":            iconStepForward,
	"icon_comment_o":               iconCommentO,
	"icon_codepen":                 iconCodepen,
	"icon_facebook":                iconFacebook,
	"icon_git":                     iconGit,
	"icon_github":                  iconGithub,
	"icon_github_alt":              IconGithubAlt,
	"icon_google":                  iconGoogle,
	"icon_google_plus":             iconGooglePlus,
	"icon_instagram":               iconInstagram,
	"icon_pinterest":               iconPinterest,
	"icon_pocket":                  iconPocket,
	"icon_twitter":                 IconTwitter,
	"icon_wordpress":               iconWordpress,
	"icon_wordpress_alt":           iconWordpressAlt,
	"icon_youtube":                 iconYoutube,
	"icon_messanger":               iconMessanger,
	"icon_activity":                iconActivity,
	"icon_bolt":                    iconBolt,
	"icon_picture_square":          iconPictureSquare,
	"icon_text_align_justify":      iconTextAlignJustify,
	"icon_add_cart":                iconAddCart,
	"icon_cage":                    IconCage,
	"icon_cart":                    iconCart,
	"icon_credit_card":             iconCreditCard,
	"icon_gift":                    iconGift,
	"icon_remove_cart":             iconRemoveCart,
	"icon_shopping_bag":            iconShoppingBag,
	"icon_truck":                   iconTruck,
	"icon_wallet":                  iconWallet,
	"icon_moon":                    IconMoon,
	"icon_sunny_o":                 IconSunnyO,
	"icon_sunrise":                 iconSunrise,
	"icon_umbrella":                iconUmbrella,
	"icon_target":                  IconTarget,
	"icon_smile_plus":              iconSmilePlus,
	"icon_smile_heart":             IconSmileHeart,
	"icon_beginner":                iconBeginner,
	"icon_train":                   iconTrain,
	"icon_donut":                   iconDonut,
	"icon_rice_cracker":            iconRiceCracker,
	"icon_apron":                   iconApron,
	"icon_octpus":                  iconOctpus,
	"icon_squid":                   iconSquid,
	"icon_bus":                     iconBus,
	"icon_car":                     iconCar,
	"icon_notice_active":           iconNoticeActive,
	"icon_notice_off":              iconNoticeOff,
	"icon_notice_on":               iconNoticeOn,
	"icon_notice_push":             iconNoticePush,
	"icon_taxi":                    iconTaxi,
	"icon_vr":                      iconVr,
	"icon_bread":                   iconBread,
	"icon_frying_pan":              iconFryingPan,
	"icon_mitarashi_dango":         iconMitarashiDango,
	"icon_tumbler_glass":           iconTumblerGlass,
	"icon_yaki_dango":              iconYakiDango,
}
