package frfpanehtml

import (
	"encoding/json"
	//	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TParams struct {
	Feedpath     string
	Step         int
	Singlemode   bool
	IndexPrefix  string
	IndexPostfix string
	LocalLink    string
}

var Params TParams

var tagReplacer *strings.Replacer

var (
	RegJsReplace *regexp.Regexp
	RegToken     *regexp.Regexp
	RegUser      *regexp.Regexp
	RegHashtag   *regexp.Regexp
	RegUrl       *regexp.Regexp
)

func initRegs() {
	RegJsReplace = regexp.MustCompile(`"([a-zA-Z]{2,})":(\d{1,})`)
	RegUser = regexp.MustCompile(`(?s)([^a-zA-Z0-9\/]|^)(@[a-zA-Z0-9\-]+)`)                                   //@username
	RegHashtag = regexp.MustCompile(`(?s)([^a-zA-Z0-9\/?'"]|^)#([^\s\x20-\x2F\x3A-\x3F\x5B-\x5E\x7B-\xBF]+)`) //hashtag
	RegUrl = regexp.MustCompile(`(?s)(https?:[^\s^\*(),\[{\]};'"><]+)`)                                       //url
}

func striptags(text string) string {
	outtext := tagReplacer.Replace(text)
	return strings.Replace(outtext, "\n", "<br>", -1)
}

func GenTplPage(template string, tstrings TXLines) string {
	tout := template
	for key, val := range tstrings {
		tout = strings.Replace(tout, "$"+key, val, -1)
	}
	return tout
}

func makespan(color, text string) string {
	return `<span style="color:` + color + `">` + text + `</span>`
}

func makeixlink(link string) string {
	return Params.IndexPrefix + link + Params.IndexPostfix
}

func highlighter(text, pen string) string {
	out := RegUser.ReplaceAll([]byte(text), []byte(`$1<span style="color:blue">$2</span>`))
	out = RegHashtag.ReplaceAll(out, []byte(`$1<span style="color:red">#$2</span>`))
	out = RegUrl.ReplaceAll(out, []byte(`<a href="$1">$1</a>`))
	if len(pen) > 0 {
		regFi := regexp.MustCompile("(" + pen + ")")
		out = regFi.ReplaceAll(out, []byte(`<span style="background-color:yellow">$1</span>`))
	}
	return string(out)
}

func (post *XPost) getgroups() TGList {
	var groups = TGList{}
	var tmpgroups = TGList{}
	for _, p := range post.PostJson.Subscribers {
		if strings.EqualFold(p.Type, "group") {
			tmpgroups[p.ID] = GroupSType{p.Username, p.IsPrivate, p.IsProtected}
		}
	}
	for _, p := range post.PostJson.Subscriptions {
		if tmpgroups[p.User].Id != "" {
			groups[p.ID] = GroupSType{tmpgroups[p.User].Id, tmpgroups[p.User].IsPrivate, tmpgroups[p.User].IsProtected}
		}
	}
	return groups
}

func (post *XPost) genCommentsHtml(pen string) string {
	cHtml := ""
	for _, p := range post.PostJson.Comments { //frfcmts.Comments
		clikes := ""
		if p.Likes != "0" {
			clikes = p.Likes
		}
		cc := GenTplPage(Templates.Comment,
			TXLines{"comment": highlighter(striptags(p.Body), pen), "author": post.usrindex[p.CreatedBy].Id, "clikes": clikes})
		//hack: insert comment time
		utime, _ := strconv.ParseInt(p.UpdatedAt, 10, 64)
		ctime := time.Unix(utime/1000, 0).Format(time.RFC822)
		cHtml += strings.Replace(cc, `<li>`, `<li title="`+ctime+`">`, -1)
	}
	return cHtml
}

func (post *XPost) genLikesHtml() string {
	//	likesHtml := ""
	if len(post.PostJson.Posts.Likes) == 0 {
		return ""
	}
	likesHtml := "<p>Likes: "
	for _, p := range post.PostJson.Posts.Likes {
		likesHtml += post.usrindex[p].Id + ", "
	}
	likesHtml = strings.TrimSuffix(likesHtml, ", ") + "</p>\n"
	return likesHtml
}

func (post *XPost) genAttachHtml() string {
	//	feedpath := Params.Feedpath
	if len(post.PostJson.Attachments) == 0 {
		return ""
	}
	attachHtml := ""
	prefix := "../media/"
	if Params.Singlemode {
		prefix = ""
	}
	for _, p := range post.PostJson.Attachments {
		file := p.ID + path.Ext(p.URL)
		if p.MediaType != "image" {
			attachHtml += "<a href=" + prefix + "media_" + file + "> Media file </a><br>\n"
		} else {
			imgsrc := prefix + `image_` + file
			attachHtml += `<a href="` + imgsrc + `">` + `<img width=233 height=175 src="` + imgsrc + `"></a><br>` + "\n"
		}
	}
	return attachHtml
}

func (post *XPost) genGroupHtml(groups TGList) string {
	ghtml := ""
	gcnt := 0
	pscnt := len(post.PostJson.Posts.PostedTo)
	for _, p := range post.PostJson.Posts.PostedTo {
		if groups[p].Id != "" {
			if groups[p].IsPrivate != "1" {
				ghtml += groups[p].Id + ":"
			} else {
				ghtml += makespan("red", groups[p].Id) + ":"
			}
			gcnt++
		}
	}
	if (pscnt > 1) && (pscnt > gcnt) {
		ghtml = "+" + ghtml
	}
	return ghtml
}

func (post *XPost) getBLineItems() (string, string) {
	createdby := post.PostJson.Posts.CreatedBy
	uuname := post.usrindex[createdby].Id
	switch {
	case post.usrindex[createdby].IsPrivate == "1":
		uuname = makespan("red", uuname)
	case post.usrindex[createdby].IsProtected == "1":
		uuname = makespan("goldenrod", uuname)
	default:
		uuname = makespan("green", uuname)
	}
	utime, _ := strconv.ParseInt(post.PostJson.Posts.CreatedAt, 10, 64)
	xtime := time.Unix(utime/1000, 0).Format(time.RFC822)
	return uuname, xtime
}

func (post *XPost) ToHtml(id, pen string) string {
	//users
	post.usrindex = TGList{}
	for _, p := range post.PostJson.Users { //frfusr.Users
		post.usrindex[p.ID] = GroupSType{p.Username, p.IsPrivate, p.IsProtected}
	}
	// groups
	ghtml := post.genGroupHtml(post.getgroups())
	//b-line
	uuname, xtime := post.getBLineItems() //auname := ghtml + uuname
	//likes
	likesHtml := post.genLikesHtml()
	//attach
	attachHtml := post.genAttachHtml()
	//comments
	commHtml := post.genCommentsHtml(pen)
	//assembly
	LLink := ""
	if len(Params.LocalLink) > 0 {
		LLink = strings.Replace(Params.LocalLink, "$id", id, -1)
	}
	tmap := TXLines{"text": highlighter(striptags(post.PostJson.Posts.Body), pen), //post
		"auname": ghtml + uuname, "id": id, "time_html": xtime, "local_link": LLink, //b-line
		"attach_html": attachHtml, "likes_html": likesHtml, "comm_html": commHtml, //attachs,likes,comments
	}
	return GenTplPage(Templates.Item, tmap)
}

func MkHtmlPage(id string, htmlText string, isIndex bool, maxeof int, feedname string, title string) string {
	nav := ""
	if isIndex {
		nav = `<nav class="pager">`
		ids, _ := strconv.Atoi(id)
		if ids != 0 {
			nav += "<a href=" + makeixlink(strconv.Itoa(ids-Params.Step)) + ` class="is-prev">Previous</a>`
		}
		if ids < maxeof {
			nav += "<a href=" + makeixlink(strconv.Itoa(ids+Params.Step)) + ` class="is-next">Next</a>`
		}
		nav += "</nav>"
	}
	outfiletext := GenTplPage(Templates.File, TXLines{"title": title, "html_text": htmlText, "pager": nav, "feedname": feedname})
	return outfiletext
}

func LoadJson(filename string) *XPost {
	npost := new(XPost)
	fbin, _ := ioutil.ReadFile(filename)
	json.Unmarshal(fbin, &npost.PostJson)
	return npost
}

func (post *XPost) TextOnly() string {
	postText := post.PostJson.Posts.Body
	for j := 0; j < len(post.PostJson.Comments); j++ {
		postText += "\n" + post.PostJson.Comments[j].Body
	}
	return postText
}

func init() {
	tagReplacer = strings.NewReplacer("<", "&lt;", ">", "&gt;")
	initRegs()
}
