package ui

// Icon holds an emoji and ASCII fallback for terminals without emoji support.
type Icon struct {
	Emoji    string
	Fallback string
}

// String returns the emoji representation. Fallback can be enabled later
// via --no-emoji flag or TERM detection without changing call sites.
func (i Icon) String() string { return i.Emoji }

// Centralized icon registry — all user-facing icons should reference these
// instead of hardcoding emoji strings in cmd/ or internal/ui/ files.
var (
	IconTask    = Icon{"📋", "[T]"}
	IconDesc    = Icon{"📝", ">"}
	IconStats   = Icon{"📊", "#"}
	IconPackage = Icon{"📦", "[P]"}
	IconSearch  = Icon{"🔍", "?"}
	IconRobot   = Icon{"🤖", "[AI]"}
	IconRocket  = Icon{"🚀", ">>"}
	IconWrench  = Icon{"🔧", "*"}
	IconBranch  = Icon{"🌿", "|-"}
	IconBolt    = Icon{"⚡", "!"}
	IconHint    = Icon{"💡", "->"}
	IconCode    = Icon{"💻", "</>"}
	IconBooks   = Icon{"📚", "[@]"}
	IconLink    = Icon{"🔗", "<->"}
	IconPlug    = Icon{"🔌", "[+]"}
	IconGlobe   = Icon{"🌐", "(o)"}
	IconSkip    = Icon{"⏭️", ">>"}
	IconTarget  = Icon{"🎯", "(*)"}
	IconFolder  = Icon{"📁", "[/]"}
	IconChat    = Icon{"💬", "\""}
	IconBook    = Icon{"📖", "[B]"}
	IconRuler   = Icon{"📐", "[R]"}

	IconOK      = Icon{"✔", "[OK]"}
	IconFail    = Icon{"✖", "[FAIL]"}
	IconWarn    = Icon{"⚠", "[WARN]"}
	IconInfo    = Icon{"ℹ", "[i]"}
	IconDone    = Icon{"✅", "[v]"}
	IconStop    = Icon{"❌", "[x]"}
	IconPartial = Icon{"◑", "[~]"}
)
