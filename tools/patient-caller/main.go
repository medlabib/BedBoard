package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type publicUIConfig struct {
	PatientCallLanguage string `json:"patientCallLanguage"`
}

type patientView struct {
	RegistrationNumber string `json:"registrationNumber"`
	Name               string `json:"name"`
	BedNumber          *int   `json:"bedNumber"`
	RoomName           string `json:"roomName"`
	BedName            string `json:"bedName"`
	Status             string `json:"status"`
}

type stateResp struct {
	Patients []patientView `json:"patients"`
}

type bbClient struct {
	base string
	http *http.Client
}

func newBBClient(base string) (*bbClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &bbClient{base: strings.TrimRight(base, "/"), http: &http.Client{Jar: jar, Timeout: 20 * time.Second}}, nil
}

func (c *bbClient) login(user, pass string) error {
	payload, _ := json.Marshal(loginReq{Username: user, Password: pass})
	resp, err := c.http.Post(c.base+"/api/auth", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *bbClient) getState() (*stateResp, error) {
	resp, err := c.http.Get(c.base + "/api/state")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("/api/state failed: HTTP %d", resp.StatusCode)
	}
	var state stateResp
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *bbClient) getPublicCallLanguage() string {
	resp, err := c.http.Get(c.base + "/api/public/ui-config")
	if err != nil {
		return "both"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "both"
	}
	var cfg publicUIConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return "both"
	}
	return normalizeCallLanguage(cfg.PatientCallLanguage)
}

func normalizeCallLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ar":
		return "ar"
	case "fr":
		return "fr"
	case "en":
		return "en"
	default:
		return "both"
	}
}

func isArabicRune(r rune) bool {
	return (r >= 0x0600 && r <= 0x06FF) || (r >= 0x0750 && r <= 0x077F) || (r >= 0x08A0 && r <= 0x08FF)
}

func isArabicText(s string) bool {
	ar := 0
	total := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			total++
			if isArabicRune(r) {
				ar++
			}
		}
	}
	return total > 0 && ar*2 >= total
}

var arToFrTunisia = map[string]string{
	"محمد": "Mohamed", "أحمد": "Ahmed", "علي": "Ali", "يوسف": "Youssef", "أيمن": "Aymen",
	"أمين": "Amine", "نبيل": "Nabil", "هيثم": "Haythem", "هيكل": "Haykel", "أنور": "Anouar",
	"سفيان": "Sofiane", "صابر": "Saber", "شكري": "Chokri", "فتحي": "Fathi", "منصف": "Moncef",
	"لطفي": "Lotfi", "نورالدين": "Noureddine", "نور الدين": "Noureddine", "عبدالرزاق": "Abderrazek", "عبد الرزاق": "Abderrazek",
	"حبيب": "Habib", "مراد": "Mourad", "رياض": "Riadh", "معز": "Moaz", "مروان": "Marouane",
	"وسيم": "Wassim", "أنيس": "Anis", "سليم": "Slim", "سامي": "Sami", "كمال": "Kamel",
	"نافع": "Nafaa", "طارق": "Tarek", "بلقاسم": "Belkacem", "الهادي": "El Hedi", "منير": "Mounir",
	"جلال": "Jalel", "بن عيسى": "Ben Aissa", "بن عمر": "Ben Omar", "بن صالح": "Ben Salah", "بن علي": "Ben Ali",
	"فاطمة": "Fatma", "سلمى": "Salma", "أسماء": "Asma", "آية": "Aya", "أية": "Aya",
	"مريم": "Meriem", "ريم": "Rym", "رانية": "Rania", "إيمان": "Imen", "أماني": "Ameni",
	"هدى": "Henda", "هناء": "Hanen", "وفاء": "Wafa", "نجلاء": "Nejla", "سعاد": "Souad",
	"حياة": "Hayet", "رجاء": "Raja", "سهام": "Sihem", "ملاك": "Malek", "مها": "Maha",
	"ليلى": "Leila", "نجاة": "Nejla", "منية": "Mounia", "روضة": "Rawdha", "كوثر": "Kawther",
	"نسرين": "Nesrine", "هديل": "Hedil", "ياسمين": "Yasmine", "إسراء": "Isra", "رحاب": "Rihab",
}

var frToArTunisia = map[string]string{
	"mohamed": "محمد", "ahmed": "أحمد", "ali": "علي", "youssef": "يوسف", "aymen": "أيمن",
	"amine": "أمين", "nabil": "نبيل", "haythem": "هيثم", "haykel": "هيكل", "anouar": "أنور",
	"sofiane": "سفيان", "saber": "صابر", "chokri": "شكري", "fathi": "فتحي", "moncef": "منصف",
	"lotfi": "لطفي", "noureddine": "نور الدين", "abderrazek": "عبد الرزاق", "habib": "حبيب", "mourad": "مراد",
	"riadh": "رياض", "moaz": "معز", "marouane": "مروان", "wassim": "وسيم", "anis": "أنيس",
	"slim": "سليم", "sami": "سامي", "kamel": "كمال", "tarek": "طارق", "mounir": "منير",
	"jalel": "جلال", "belkacem": "بلقاسم", "el hedi": "الهادي", "ben aissa": "بن عيسى", "ben omar": "بن عمر",
	"fatma": "فاطمة", "salma": "سلمى", "asma": "أسماء", "aya": "آية", "meriem": "مريم",
	"rym": "ريم", "rania": "رانية", "imen": "إيمان", "ameni": "أماني", "henda": "هدى",
	"hanen": "هناء", "wafa": "وفاء", "nejla": "نجلاء", "souad": "سعاد", "hayet": "حياة",
	"raja": "رجاء", "sihem": "سهام", "malek": "ملاك", "maha": "مها", "leila": "ليلى",
	"mounia": "منية", "rawdha": "روضة", "kawther": "كوثر", "nesrine": "نسرين", "yasmine": "ياسمين",
}

var arLetterMap = map[rune]string{
	'ا': "a", 'أ': "a", 'إ': "i", 'آ': "a", 'ى': "a", 'ٱ': "a",
	'ب': "b", 'ت': "t", 'ث': "th", 'ج': "j", 'ح': "h", 'خ': "kh",
	'د': "d", 'ذ': "dh", 'ر': "r", 'ز': "z", 'س': "s", 'ش': "ch",
	'ص': "s", 'ض': "d", 'ط': "t", 'ظ': "dh", 'ع': "", 'غ': "gh",
	'ف': "f", 'ق': "k", 'ك': "k", 'ل': "l", 'م': "m", 'ن': "n", 'ه': "h", 'و': "ou", 'ي': "i", 'ة': "a",
}

func arabicToLatin(word string) string {
	if v, ok := arToFrTunisia[word]; ok {
		return v
	}
	var out strings.Builder
	for _, r := range word {
		if v, ok := arLetterMap[r]; ok {
			out.WriteString(v)
			continue
		}
		if !isArabicRune(r) {
			out.WriteRune(r)
		}
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return word
	}
	r := []rune(text)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func latinToArabic(word string) string {
	key := strings.ToLower(strings.TrimSpace(word))
	if v, ok := frToArTunisia[key]; ok {
		return v
	}
	return word
}

func resolveBilingualName(name string) (frName, arName string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Patient inconnu", "مريض غير معروف"
	}
	parts := strings.Fields(name)
	if isArabicText(name) {
		fr := make([]string, 0, len(parts))
		for _, p := range parts {
			fr = append(fr, arabicToLatin(p))
		}
		return strings.Join(fr, " "), name
	}
	ar := make([]string, 0, len(parts))
	for _, p := range parts {
		ar = append(ar, latinToArabic(p))
	}
	return name, strings.Join(ar, " ")
}

func statusAllowed(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "arrived" || s == "triaged" || s == "waiting" || s == "assigned" || s == "in_exam"
}

func buildAnnouncement(language, nameFR, nameAR, place string) string {
	switch language {
	case "ar":
		if place == "" {
			return fmt.Sprintf("الرجاء من المريض %s التوجه إلى مكتب الاستقبال.", nameAR)
		}
		return fmt.Sprintf("الرجاء من المريض %s التوجه إلى %s.", nameAR, place)
	case "fr":
		if place == "" {
			return fmt.Sprintf("Le patient %s est attendu a l'accueil.", nameFR)
		}
		return fmt.Sprintf("Le patient %s est attendu a %s.", nameFR, place)
	case "en":
		if place == "" {
			return fmt.Sprintf("Patient %s, please proceed to reception.", nameFR)
		}
		return fmt.Sprintf("Patient %s, please proceed to %s.", nameFR, place)
	default:
		if place == "" {
			return fmt.Sprintf("Le patient %s est attendu a l'accueil. الرجاء من المريض %s التوجه إلى مكتب الاستقبال.", nameFR, nameAR)
		}
		return fmt.Sprintf("Le patient %s est attendu a %s. الرجاء من المريض %s التوجه إلى %s.", nameFR, place, nameAR, place)
	}
}

func speak(text, lang string, rate int, dry bool) {
	fmt.Printf("[%s] %s\n", strings.ToUpper(lang), text)
	if dry {
		return
	}
	switch runtime.GOOS {
	case "linux":
		voice := "fr"
		if lang == "ar" {
			voice = "ar"
		} else if lang == "en" {
			voice = "en"
		}
		bin := "espeak-ng"
		if _, err := exec.LookPath(bin); err != nil {
			bin = "espeak"
		}
		cmd := exec.Command(bin, "-v", voice, "-s", fmt.Sprintf("%d", rate), text)
		_ = cmd.Run()
	case "darwin":
		voice := "Thomas"
		if lang == "ar" {
			voice = "Tarik"
		} else if lang == "en" {
			voice = "Alex"
		}
		cmd := exec.Command("say", "-v", voice, "-r", fmt.Sprintf("%d", rate), text)
		_ = cmd.Run()
	case "windows":
		escaped := strings.ReplaceAll(text, "'", "''")
		script := fmt.Sprintf(`Add-Type -AssemblyName System.Speech;$s=New-Object System.Speech.Synthesis.SpeechSynthesizer;$s.Speak('%s')`, escaped)
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
		_ = cmd.Run()
	}
}

func main() {
	host := flag.String("host", "http://localhost:8080", "BedBoard base URL")
	user := flag.String("user", "admin", "BedBoard username")
	pass := flag.String("pass", "", "BedBoard password (or BEDBOARD_PASSWORD)")
	lang := flag.String("lang", "auto", "Call language: auto|both|ar|fr|en")
	reg := flag.String("reg", "", "Call only one registration number")
	status := flag.String("status", "", "Optional status filter (arrived|triaged|waiting|assigned|in_exam)")
	dry := flag.Bool("dry", false, "Print announcements only")
	loop := flag.Bool("loop", false, "Loop mode")
	interval := flag.Duration("interval", 45*time.Second, "Loop poll interval")
	rate := flag.Int("rate", 140, "TTS speed")
	flag.Parse()

	if *pass == "" {
		*pass = os.Getenv("BEDBOARD_PASSWORD")
	}
	if *pass == "" {
		log.Fatal("password required: -pass or BEDBOARD_PASSWORD")
	}

	client, err := newBBClient(*host)
	if err != nil {
		log.Fatalf("client init: %v", err)
	}
	if err := client.login(*user, *pass); err != nil {
		log.Fatalf("login failed: %v", err)
	}

	selectedLang := normalizeCallLanguage(*lang)
	if strings.EqualFold(*lang, "auto") {
		selectedLang = client.getPublicCallLanguage()
	}
	log.Printf("patient calling language: %s", selectedLang)

	announced := map[string]bool{}
	poll := func() {
		state, err := client.getState()
		if err != nil {
			log.Printf("poll error: %v", err)
			return
		}
		patients := make([]patientView, 0, len(state.Patients))
		for _, p := range state.Patients {
			if strings.TrimSpace(*reg) != "" && p.RegistrationNumber != strings.TrimSpace(*reg) {
				continue
			}
			if strings.TrimSpace(*status) != "" {
				if strings.ToLower(strings.TrimSpace(p.Status)) != strings.ToLower(strings.TrimSpace(*status)) {
					continue
				}
			} else if !statusAllowed(p.Status) {
				continue
			}
			if *loop && announced[p.RegistrationNumber] {
				continue
			}
			patients = append(patients, p)
		}

		sort.SliceStable(patients, func(i, j int) bool {
			return patients[i].RegistrationNumber < patients[j].RegistrationNumber
		})

		for _, p := range patients {
			frName, arName := resolveBilingualName(p.Name)
			place := strings.TrimSpace(strings.Join([]string{p.RoomName, p.BedName}, " - "))
			if place == "-" {
				place = ""
			}

			switch selectedLang {
			case "both":
				speak(buildAnnouncement("fr", frName, arName, place), "fr", *rate, *dry)
				time.Sleep(900 * time.Millisecond)
				speak(buildAnnouncement("ar", frName, arName, place), "ar", *rate, *dry)
			default:
				speak(buildAnnouncement(selectedLang, frName, arName, place), selectedLang, *rate, *dry)
			}
			announced[p.RegistrationNumber] = true
			time.Sleep(1600 * time.Millisecond)
		}
	}

	poll()
	if !*loop {
		return
	}
	log.Printf("loop mode active: every %s", interval.String())
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for range ticker.C {
		poll()
	}
}
