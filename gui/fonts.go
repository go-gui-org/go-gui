package gui

import (
	_ "embed"
	"slices"
)

// FontVariants holds paths of font files used by the GUI.
type FontVariants struct {
	Normal string
	Bold   string
	Italic string
	Mono   string
}

// Font name constants.
const (
	BaseFontName = defaultFontFamily
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
var AppFontPaths []string

// RegisterAppFont appends path to AppFontPaths if not already
// present. Safe to call multiple times with the same path.
func RegisterAppFont(path string) {
	if slices.Contains(AppFontPaths, path) {
		return
	}
	AppFontPaths = append(AppFontPaths, path)
}

// Icon constants — Feather icon font Unicode mappings.
const (
	IconArrowDown              = "\uf100"
	IconArrowLeft              = "\uf101"
	IconArrowRight             = "\uf102"
	IconArrowUp                = "\uf103"
	IconArtboard               = "\uf104"
	IconBar                    = "\uf105"
	IconBarChart               = "\uf106"
	IconBeer                   = "\uf107"
	IconBell                   = "\uf108"
	IconBook                   = "\uf109"
	IconBrowser                = "\uf10b"
	IconBrush                  = "\uf10c"
	IconBug                    = "\uf10d"
	IconBuilding               = "\uf10e"
	IconCalendar               = "\uf10f"
	IconCamera                 = "\uf110"
	IconCheck                  = "\uf111"
	IconClock                  = "\uf113"
	IconClose                  = "\uf114"
	IconCloud                  = "\uf115"
	IconCocktail               = "\uf116"
	IconCode                   = "\uf117"
	IconColumns                = "\uf118"
	IconComment                = "\uf119"
	IconCommenting             = "\uf11a"
	IconComments               = "\uf11b"
	IconDesktop                = "\uf11d"
	IconDiamond                = "\uf11e"
	IconDisabled               = "\uf11f"
	IconDownload               = "\uf120"
	IconDropDown               = "\uf121"
	IconDropLeft               = "\uf122"
	IconDropRight              = "\uf123"
	IconDropUp                 = "\uf124"
	IconElipsisH               = "\uf125"
	IconElipsisV               = "\uf126"
	IconEye                    = "\uf127"
	IconFeed                   = "\uf128"
	IconFlag                   = "\uf129"
	IconFolder                 = "\uf12a"
	IconFork                   = "\uf12b"
	IconGlobe                  = "\uf12c"
	IconHash                   = "\uf12d"
	IconHeart                  = "\uf12e"
	IconHome                   = "\uf12f"
	IconInfo                   = "\uf130"
	IconKey                    = "\uf131"
	IconKeyboard               = "\uf132"
	IconLaptop                 = "\uf133"
	IconLayout                 = "\uf134"
	IconLineChart              = "\uf135"
	IconLink                   = "\uf136"
	IconLinkExternal           = "\uf137"
	IconLocation               = "\uf138"
	IconLock                   = "\uf139"
	IconLogin                  = "\uf13a"
	IconLogout                 = "\uf13b"
	IconMail                   = "\uf13c"
	IconMedal                  = "\uf13d"
	IconMegaphone              = "\uf13e"
	IconMinus                  = "\uf140"
	IconMobile                 = "\uf141"
	IconMouse                  = "\uf142"
	IconPencil                 = "\uf144"
	IconPhone                  = "\uf145"
	IconPieChart               = "\uf146"
	IconPizza                  = "\uf147"
	IconPlus                   = "\uf148"
	IconPrototype              = "\uf149"
	IconQuestion               = "\uf14a"
	IconQuoteLeft              = "\uf14b"
	IconQuoteRight             = "\uf14c"
	IconRocket                 = "\uf14d"
	IconSearch                 = "\uf14e"
	IconShare                  = "\uf14f"
	IconSitemap                = "\uf150"
	IconStar                   = "\uf151"
	IconTablet                 = "\uf152"
	IconTag                    = "\uf153"
	IconTerminal               = "\uf154"
	IconTicket                 = "\uf155"
	IconTiled                  = "\uf156"
	IconTrash                  = "\uf157"
	IconTrophy                 = "\uf158"
	IconUpload                 = "\uf159"
	IconUser                   = "\uf15a"
	IconUserPlus               = "\uf15b"
	IconUsers                  = "\uf15c"
	IconVector                 = "\uf15d"
	IconVideo                  = "\uf15e"
	IconWarning                = "\uf15f"
	IconWineGlass              = "\uf161"
	IconWrench                 = "\uf162"
	IconBirthdayCake           = "\uf163"
	IconMention                = "\uf164"
	IconPalette                = "\uf165"
	IconCoffee                 = "\uf166"
	IconHeartO                 = "\uf167"
	IconStarO                  = "\uf168"
	IconUnlock                 = "\uf169"
	IconSearchMinus            = "\uf16a"
	IconSearchPlus             = "\uf16b"
	IconUserMinus              = "\uf16c"
	IconMapIcon                = "\uf16d"
	IconExport                 = "\uf16e"
	IconImport                 = "\uf16f"
	IconBookmark               = "\uf170"
	IconPrint                  = "\uf171"
	IconShield                 = "\uf172"
	IconFilter                 = "\uf173"
	IconFeather                = "\uf174"
	IconMusic                  = "\uf175"
	IconFolderOpen             = "\uf176"
	IconMagic                  = "\uf177"
	IconPaperPlane             = "\uf178"
	IconBold                   = "\uf179"
	IconItalic                 = "\uf17a"
	IconTextSize               = "\uf17b"
	IconListBullet             = "\uf17c"
	IconListOrder              = "\uf17d"
	IconListTask               = "\uf17e"
	IconEdit                   = "\uf17f"
	IconBackward               = "\uf180"
	IconCompress               = "\uf181"
	IconEject                  = "\uf182"
	IconExpand                 = "\uf183"
	IconFastBackward           = "\uf184"
	IconFastForward            = "\uf185"
	IconForward                = "\uf186"
	IconPause                  = "\uf187"
	IconPlay                   = "\uf188"
	IconRandom                 = "\uf189"
	IconStop                   = "\uf18b"
	IconLayer                  = "\uf18c"
	IconHeadphone              = "\uf18e"
	IconPlug                   = "\uf18f"
	IconUsb                    = "\uf190"
	IconGamepad                = "\uf191"
	IconLoop                   = "\uf192"
	IconSync                   = "\uf194"
	IconAlignCenter            = "\uf195"
	IconAlignLeft              = "\uf196"
	IconAlignRight             = "\uf197"
	IconAppMenu                = "\uf198"
	IconAudioPlayer            = "\uf199"
	IconCheckCircle            = "\uf19a"
	IconCheckCircleO           = "\uf19b"
	IconCheckVerified          = "\uf19c"
	IconCutlery                = "\uf19d"
	IconDeleteLink             = "\uf19e"
	IconDocument               = "\uf19f"
	IconEqualizer              = "\uf1a0"
	IconFileExcel              = "\uf1a2"
	IconFilePowerpoint         = "\uf1a3"
	IconFileWord               = "\uf1a4"
	IconGear                   = "\uf1a5"
	IconInsertLink             = "\uf1a6"
	IconKitchenCooker          = "\uf1a7"
	IconMoney                  = "\uf1a8"
	IconPicture                = "\uf1a9"
	IconPot                    = "\uf1aa"
	IconSpeaker                = "\uf1ab"
	IconTable                  = "\uf1ac"
	IconTimeline               = "\uf1ad"
	IconUnderline              = "\uf1ae"
	IconWatch                  = "\uf1af"
	IconWatchAlt               = "\uf1b0"
	IconFile                   = "\uf1b1"
	IconFileAudio              = "\uf1b2"
	IconFileImage              = "\uf1b3"
	IconFileMovie              = "\uf1b4"
	IconFileZip                = "\uf1b5"
	IconAngry                  = "\uf1b6"
	IconCry                    = "\uf1b7"
	IconDisappointed           = "\uf1b8"
	IconFrowing                = "\uf1b9"
	IconOpenMouth              = "\uf1ba"
	IconRage                   = "\uf1bb"
	IconSmile                  = "\uf1bc"
	IconSmileAlt               = "\uf1bd"
	IconTired                  = "\uf1be"
	IconAlignBottom            = "\uf1bf"
	IconAlignTop               = "\uf1c0"
	IconAlignVertically        = "\uf1c1"
	IconCrop                   = "\uf1c2"
	IconDifference             = "\uf1c3"
	IconDistributeVertically   = "\uf1c5"
	IconEraser                 = "\uf1c6"
	IconIntersect              = "\uf1c7"
	IconMask                   = "\uf1c8"
	IconScale                  = "\uf1c9"
	IconSubtract               = "\uf1ca"
	IconTextAlignCenter        = "\uf1cb"
	IconTextAlignLeft          = "\uf1cc"
	IconTextAlignRight         = "\uf1cd"
	IconUnion                  = "\uf1ce"
	IconDistributeHorizontally = "\uf1cf"
	IconStepBackward           = "\uf1d0"
	IconStepForward            = "\uf1d1"
	IconCommentO               = "\uf1d2"
	IconCodepen                = "\uf1d3"
	IconFacebook               = "\uf1d4"
	IconGit                    = "\uf1d5"
	IconGithub                 = "\uf1d6"
	IconGithubAlt              = "\uf1d7"
	IconGoogle                 = "\uf1d8"
	IconGooglePlus             = "\uf1d9"
	IconInstagram              = "\uf1da"
	IconPinterest              = "\uf1db"
	IconPocket                 = "\uf1dc"
	IconTwitter                = "\uf1dd"
	IconWordpress              = "\uf1de"
	IconWordpressAlt           = "\uf1df"
	IconYoutube                = "\uf1e0"
	IconMessanger              = "\uf1e1"
	IconActivity               = "\uf1e2"
	IconBolt                   = "\uf1e3"
	IconPictureSquare          = "\uf1e4"
	IconTextAlignJustify       = "\uf1e5"
	IconAddCart                = "\uf1e6"
	IconCage                   = "\uf1e7"
	IconCart                   = "\uf1e8"
	IconCreditCard             = "\uf1e9"
	IconGift                   = "\uf1ea"
	IconRemoveCart             = "\uf1eb"
	IconShoppingBag            = "\uf1ec"
	IconTruck                  = "\uf1ed"
	IconWallet                 = "\uf1ee"
	IconMoon                   = "\uf1ef"
	IconSunnyO                 = "\uf1f0"
	IconSunrise                = "\uf1f1"
	IconUmbrella               = "\uf1f2"
	IconTarget                 = "\uf1f3"
	IconSmilePlus              = "\uf1f5"
	IconSmileHeart             = "\uf1f6"
	IconBeginner               = "\uf1f7"
	IconTrain                  = "\uf1f8"
	IconDonut                  = "\uf1f9"
	IconRiceCracker            = "\uf1fa"
	IconApron                  = "\uf1fb"
	IconOctpus                 = "\uf1fc"
	IconSquid                  = "\uf1fd"
	IconBus                    = "\uf1fe"
	IconCar                    = "\uf1ff"
	IconNoticeActive           = "\uf200"
	IconNoticeOff              = "\uf201"
	IconNoticeOn               = "\uf202"
	IconNoticePush             = "\uf203"
	IconTaxi                   = "\uf204"
	IconVr                     = "\uf205"
	IconBread                  = "\uf206"
	IconFryingPan              = "\uf207"
	IconMitarashiDango         = "\uf208"
	IconTumblerGlass           = "\uf209"
	IconYakiDango              = "\uf20a"
)

// IconLookup maps V-style snake_case icon names to Unicode values.
var IconLookup = map[string]string{
	"icon_arrow_down":              IconArrowDown,
	"icon_arrow_left":              IconArrowLeft,
	"icon_arrow_right":             IconArrowRight,
	"icon_arrow_up":                IconArrowUp,
	"icon_artboard":                IconArtboard,
	"icon_bar":                     IconBar,
	"icon_bar_chart":               IconBarChart,
	"icon_beer":                    IconBeer,
	"icon_bell":                    IconBell,
	"icon_book":                    IconBook,
	"icon_browser":                 IconBrowser,
	"icon_brush":                   IconBrush,
	"icon_bug":                     IconBug,
	"icon_building":                IconBuilding,
	"icon_calendar":                IconCalendar,
	"icon_camera":                  IconCamera,
	"icon_check":                   IconCheck,
	"icon_clock":                   IconClock,
	"icon_close":                   IconClose,
	"icon_cloud":                   IconCloud,
	"icon_cocktail":                IconCocktail,
	"icon_code":                    IconCode,
	"icon_columns":                 IconColumns,
	"icon_comment":                 IconComment,
	"icon_commenting":              IconCommenting,
	"icon_comments":                IconComments,
	"icon_desktop":                 IconDesktop,
	"icon_diamond":                 IconDiamond,
	"icon_disabled":                IconDisabled,
	"icon_download":                IconDownload,
	"icon_drop_down":               IconDropDown,
	"icon_drop_left":               IconDropLeft,
	"icon_drop_right":              IconDropRight,
	"icon_drop_up":                 IconDropUp,
	"icon_elipsis_h":               IconElipsisH,
	"icon_elipsis_v":               IconElipsisV,
	"icon_eye":                     IconEye,
	"icon_feed":                    IconFeed,
	"icon_flag":                    IconFlag,
	"icon_folder":                  IconFolder,
	"icon_fork":                    IconFork,
	"icon_globe":                   IconGlobe,
	"icon_hash":                    IconHash,
	"icon_heart":                   IconHeart,
	"icon_home":                    IconHome,
	"icon_info":                    IconInfo,
	"icon_key":                     IconKey,
	"icon_keyboard":                IconKeyboard,
	"icon_laptop":                  IconLaptop,
	"icon_layout":                  IconLayout,
	"icon_line_chart":              IconLineChart,
	"icon_link":                    IconLink,
	"icon_link_external":           IconLinkExternal,
	"icon_location":                IconLocation,
	"icon_lock":                    IconLock,
	"icon_login":                   IconLogin,
	"icon_logout":                  IconLogout,
	"icon_mail":                    IconMail,
	"icon_medal":                   IconMedal,
	"icon_megaphone":               IconMegaphone,
	"icon_minus":                   IconMinus,
	"icon_mobile":                  IconMobile,
	"icon_mouse":                   IconMouse,
	"icon_pencil":                  IconPencil,
	"icon_phone":                   IconPhone,
	"icon_pie_chart":               IconPieChart,
	"icon_pizza":                   IconPizza,
	"icon_plus":                    IconPlus,
	"icon_prototype":               IconPrototype,
	"icon_question":                IconQuestion,
	"icon_quote_left":              IconQuoteLeft,
	"icon_quote_right":             IconQuoteRight,
	"icon_rocket":                  IconRocket,
	"icon_search":                  IconSearch,
	"icon_share":                   IconShare,
	"icon_sitemap":                 IconSitemap,
	"icon_star":                    IconStar,
	"icon_tablet":                  IconTablet,
	"icon_tag":                     IconTag,
	"icon_terminal":                IconTerminal,
	"icon_ticket":                  IconTicket,
	"icon_tiled":                   IconTiled,
	"icon_trash":                   IconTrash,
	"icon_trophy":                  IconTrophy,
	"icon_upload":                  IconUpload,
	"icon_user":                    IconUser,
	"icon_user_plus":               IconUserPlus,
	"icon_users":                   IconUsers,
	"icon_vector":                  IconVector,
	"icon_video":                   IconVideo,
	"icon_warning":                 IconWarning,
	"icon_wine_glass":              IconWineGlass,
	"icon_wrench":                  IconWrench,
	"icon_birthday_cake":           IconBirthdayCake,
	"icon_mention":                 IconMention,
	"icon_palette":                 IconPalette,
	"icon_coffee":                  IconCoffee,
	"icon_heart_o":                 IconHeartO,
	"icon_star_o":                  IconStarO,
	"icon_unlock":                  IconUnlock,
	"icon_search_minus":            IconSearchMinus,
	"icon_search_plus":             IconSearchPlus,
	"icon_user_minus":              IconUserMinus,
	"icon_map":                     IconMapIcon,
	"icon_export":                  IconExport,
	"icon_import":                  IconImport,
	"icon_bookmark":                IconBookmark,
	"icon_print":                   IconPrint,
	"icon_shield":                  IconShield,
	"icon_filter":                  IconFilter,
	"icon_feather":                 IconFeather,
	"icon_music":                   IconMusic,
	"icon_folder_open":             IconFolderOpen,
	"icon_magic":                   IconMagic,
	"icon_paper_plane":             IconPaperPlane,
	"icon_bold":                    IconBold,
	"icon_italic":                  IconItalic,
	"icon_text_size":               IconTextSize,
	"icon_list_bullet":             IconListBullet,
	"icon_list_order":              IconListOrder,
	"icon_list_task":               IconListTask,
	"icon_edit":                    IconEdit,
	"icon_backward":                IconBackward,
	"icon_compress":                IconCompress,
	"icon_eject":                   IconEject,
	"icon_expand":                  IconExpand,
	"icon_fast_backward":           IconFastBackward,
	"icon_fast_forward":            IconFastForward,
	"icon_forward":                 IconForward,
	"icon_pause":                   IconPause,
	"icon_play":                    IconPlay,
	"icon_random":                  IconRandom,
	"icon_stop":                    IconStop,
	"icon_layer":                   IconLayer,
	"icon_headphone":               IconHeadphone,
	"icon_plug":                    IconPlug,
	"icon_usb":                     IconUsb,
	"icon_gamepad":                 IconGamepad,
	"icon_loop":                    IconLoop,
	"icon_sync":                    IconSync,
	"icon_align_center":            IconAlignCenter,
	"icon_align_left":              IconAlignLeft,
	"icon_align_right":             IconAlignRight,
	"icon_app_menu":                IconAppMenu,
	"icon_audio_player":            IconAudioPlayer,
	"icon_check_circle":            IconCheckCircle,
	"icon_check_circle_o":          IconCheckCircleO,
	"icon_check_verified":          IconCheckVerified,
	"icon_cutlery":                 IconCutlery,
	"icon_delete_link":             IconDeleteLink,
	"icon_document":                IconDocument,
	"icon_equalizer":               IconEqualizer,
	"icon_file_excel":              IconFileExcel,
	"icon_file_powerpoint":         IconFilePowerpoint,
	"icon_file_word":               IconFileWord,
	"icon_gear":                    IconGear,
	"icon_insert_link":             IconInsertLink,
	"icon_kitchen_cooker":          IconKitchenCooker,
	"icon_money":                   IconMoney,
	"icon_picture":                 IconPicture,
	"icon_pot":                     IconPot,
	"icon_speaker":                 IconSpeaker,
	"icon_table":                   IconTable,
	"icon_timeline":                IconTimeline,
	"icon_underline":               IconUnderline,
	"icon_watch":                   IconWatch,
	"icon_watch_alt":               IconWatchAlt,
	"icon_file":                    IconFile,
	"icon_file_audio":              IconFileAudio,
	"icon_file_image":              IconFileImage,
	"icon_file_movie":              IconFileMovie,
	"icon_file_zip":                IconFileZip,
	"icon_angry":                   IconAngry,
	"icon_cry":                     IconCry,
	"icon_disappointed":            IconDisappointed,
	"icon_frowing":                 IconFrowing,
	"icon_open_mouth":              IconOpenMouth,
	"icon_rage":                    IconRage,
	"icon_smile":                   IconSmile,
	"icon_smile_alt":               IconSmileAlt,
	"icon_tired":                   IconTired,
	"icon_align_bottom":            IconAlignBottom,
	"icon_align_top":               IconAlignTop,
	"icon_align_vertically":        IconAlignVertically,
	"icon_crop":                    IconCrop,
	"icon_difference":              IconDifference,
	"icon_distribute_vertically":   IconDistributeVertically,
	"icon_eraser":                  IconEraser,
	"icon_intersect":               IconIntersect,
	"icon_mask":                    IconMask,
	"icon_scale":                   IconScale,
	"icon_subtract":                IconSubtract,
	"icon_text_align_center":       IconTextAlignCenter,
	"icon_text_align_left":         IconTextAlignLeft,
	"icon_text_align_right":        IconTextAlignRight,
	"icon_union":                   IconUnion,
	"icon_distribute_horizontally": IconDistributeHorizontally,
	"icon_step_backward":           IconStepBackward,
	"icon_step_forward":            IconStepForward,
	"icon_comment_o":               IconCommentO,
	"icon_codepen":                 IconCodepen,
	"icon_facebook":                IconFacebook,
	"icon_git":                     IconGit,
	"icon_github":                  IconGithub,
	"icon_github_alt":              IconGithubAlt,
	"icon_google":                  IconGoogle,
	"icon_google_plus":             IconGooglePlus,
	"icon_instagram":               IconInstagram,
	"icon_pinterest":               IconPinterest,
	"icon_pocket":                  IconPocket,
	"icon_twitter":                 IconTwitter,
	"icon_wordpress":               IconWordpress,
	"icon_wordpress_alt":           IconWordpressAlt,
	"icon_youtube":                 IconYoutube,
	"icon_messanger":               IconMessanger,
	"icon_activity":                IconActivity,
	"icon_bolt":                    IconBolt,
	"icon_picture_square":          IconPictureSquare,
	"icon_text_align_justify":      IconTextAlignJustify,
	"icon_add_cart":                IconAddCart,
	"icon_cage":                    IconCage,
	"icon_cart":                    IconCart,
	"icon_credit_card":             IconCreditCard,
	"icon_gift":                    IconGift,
	"icon_remove_cart":             IconRemoveCart,
	"icon_shopping_bag":            IconShoppingBag,
	"icon_truck":                   IconTruck,
	"icon_wallet":                  IconWallet,
	"icon_moon":                    IconMoon,
	"icon_sunny_o":                 IconSunnyO,
	"icon_sunrise":                 IconSunrise,
	"icon_umbrella":                IconUmbrella,
	"icon_target":                  IconTarget,
	"icon_smile_plus":              IconSmilePlus,
	"icon_smile_heart":             IconSmileHeart,
	"icon_beginner":                IconBeginner,
	"icon_train":                   IconTrain,
	"icon_donut":                   IconDonut,
	"icon_rice_cracker":            IconRiceCracker,
	"icon_apron":                   IconApron,
	"icon_octpus":                  IconOctpus,
	"icon_squid":                   IconSquid,
	"icon_bus":                     IconBus,
	"icon_car":                     IconCar,
	"icon_notice_active":           IconNoticeActive,
	"icon_notice_off":              IconNoticeOff,
	"icon_notice_on":               IconNoticeOn,
	"icon_notice_push":             IconNoticePush,
	"icon_taxi":                    IconTaxi,
	"icon_vr":                      IconVr,
	"icon_bread":                   IconBread,
	"icon_frying_pan":              IconFryingPan,
	"icon_mitarashi_dango":         IconMitarashiDango,
	"icon_tumbler_glass":           IconTumblerGlass,
	"icon_yaki_dango":              IconYakiDango,
}
