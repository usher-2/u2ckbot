package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	pb "github.com/usher2/u2ckbot/msg"
)

const (
	TBLOCK_URL = iota
	TBLOCK_HTTPS
	TBLOCK_DOMAIN
	TBLOCK_MASK
	TBLOCK_IP
)

const PRINT_LIMIT = 5

const MAX_TIMESTAMP int64 = 1<<63 - 1

const (
	DTIME12 = 12 * 60 * 60
	DTIME3  = 3 * 60 * 60
)

const (
	_ = iota
	OFFSET_CONTENT
)

type TPagination struct {
	Tag, Count int
}

type TReason struct {
	Id     int32
	Aggr   []string
	Ip     []string
	Url    []string
	Domain []string
}

func printUpToDate(t int64) string {
	var r rune
	d := time.Now().Unix() - t
	switch {
	case d > DTIME12:
		r = 0x2b55
	case d > DTIME3:
		r = 0x000026a0
	default:
		r = 0x2705
	}
	return fmt.Sprintf("\n%c _Данные синхронизированы:_ %s\n", r, time.Unix(t, 0).In(time.FixedZone("UTC+3", 3*60*60)).Format(time.RFC3339))
}

func constructBasis(content *TContent) (res string) {
	basis := "[ст. 15.1](http://www.consultant.ru/document/cons_doc_LAW_61798/38c8ea666d27d9dc12b078c556e316e90248f551/), общая"
	if content.EntryType == 1 && content.Decision.Org == "Генпрокуратура" {
		basis = "[ст. 15.1-1](http://www.consultant.ru/document/cons_doc_LAW_61798/079aac275ffc6cea954b19c5b177a547b94f3c48/), неуважение"
	} else if content.EntryType == 2 {
		basis = "[ст. 15.2](http://www.consultant.ru/document/cons_doc_LAW_61798/1f316dc4a18023edcd030bc6591c4dd8b4f841dc/), правообладание"
	} else if content.EntryType == 3 {
		basis = "[ст. 15.3](http://www.consultant.ru/document/cons_doc_LAW_61798/34547c9b6ddb60cebd0a67593943fd9ef64ebdd0/), мятеж и фейки"
	} else if content.EntryType == 4 {
		basis = "[ст. 15.4](http://www.consultant.ru/document/cons_doc_LAW_61798/96723dcd9be73473a978013263f16f42cd8cd53d/), ОРИ"
	} else if content.EntryType == 5 && content.Decision.Org == "Мосгорсуд" {
		basis = "[ст. 15.6](http://www.consultant.ru/document/cons_doc_LAW_61798/c7c4ad36689c46c7e8a3ab49c9db8ccbc7c82920/), вечная"
	} else if content.EntryType == 5 && content.Decision.Org == "Минкомсвязь" {
		basis = "[ст. 15.6-1](http://www.consultant.ru/document/cons_doc_LAW_61798/c7c4ad36689c46c7e8a3ab49c9db8ccbc7c82920/), вечная зеркал"
	} else if content.EntryType == 6 {
		basis = "[ст. 15.5](http://www.consultant.ru/document/cons_doc_LAW_61798/98228cbe6565abbe55d0842a7e8593012c3449ea/), персональные данные"
	} else if content.EntryType == 7 {
		basis = "[ст. 15.8](http://www.consultant.ru/document/cons_doc_LAW_61798/1a807328c80a540bd0bb724927d6e774595431dc/), VPN"
	}
	return basis
}

func constructContentResult(a []*pb.Content) (res string) {
	var mass string
	var oldest int64 = MAX_TIMESTAMP
	if len(a) == 0 {
		return
	}
	for _, packet := range a {
		content := TContent{}
		err := json.Unmarshal(packet.Pack, &content)
		if err != nil {
			Error.Printf("Упс!!! %s\n", err)
			continue
		}
		if packet.RegistryUpdateTime < oldest {
			oldest = packet.RegistryUpdateTime
		}
		Debug.Printf("%v\n", content)
		if len(content.Subnet4)+len(content.Subnet6) > 0 && packet.BlockType == TBLOCK_IP {
			mass = "\U0001f4a5\U0001f4a5\U0001f4a5 Решение о «ковровой» блокировке!\n\n"
		}
		bt := ""
		switch packet.BlockType {
		case TBLOCK_URL:
			bt = "\U000026d4 (url) "
		case TBLOCK_HTTPS:
			bt = "\U0001f4db (https) "
		case TBLOCK_DOMAIN:
			bt = "\U0001f6ab (domain) "
		case TBLOCK_MASK:
			bt = "\U0001f506 (wildcard) "
		case TBLOCK_IP:
			bt = "\u274c (ip) "
		}
		dcs := fmt.Sprintf("%s %s %s", content.Decision.Org, content.Decision.Number, content.Decision.Date)
		basis := constructBasis(&content)
		res += fmt.Sprintf("%s /n\\_%d %s (%s)\n", bt, content.Id, dcs, basis)
		res += fmt.Sprintf("внесено: %s\n", time.Unix(content.IncludeTime, 0).In(time.FixedZone("UTC+3", 3*60*60)).Format(time.RFC3339))
		for i, d := range content.Domain {
			if i >= PRINT_LIMIT {
				if len(content.Domain)-i == 1 {
					res += fmt.Sprintf("... и ещё %d домен\n", len(content.Domain)-i)
				} else if len(content.Domain)-i > 1 && len(content.Domain)-i < 5 {
					res += fmt.Sprintf("... и %d домена\n", len(content.Domain)-i)
				} else {
					res += fmt.Sprintf("... и %d доменов\n", len(content.Domain)-i)
				}
				break
			}
			res += fmt.Sprintf("  domain: %s\n", Sanitize(d.Domain))
		}
		for i, u := range content.Url {
			if i >= PRINT_LIMIT {
				res += fmt.Sprintf("... и ещё %d URL\n", len(content.Url)-i)
				break
			}
			res += fmt.Sprintf("  url: %s\n", Sanitize(u.Url))
		}
		for i, ip := range content.Ip4 {
			if i >= PRINT_LIMIT {
				if len(content.Ip4)-i == 1 {
					res += fmt.Sprintf("... и %d IP-адрес\n", len(content.Ip4)-i)
				} else if len(content.Ip4)-i > 1 && len(content.Ip4)-i < 5 {
					res += fmt.Sprintf("... и ещё %d IP-адреса\n", len(content.Ip4)-i)
				} else {
					res += fmt.Sprintf("... и ещё %d IP-адресов\n", len(content.Ip4)-i)
				}
				break
			}
			res += fmt.Sprintf("  IP: %s\n", int2Ip4(ip.Ip4))
		}
		for i, ip := range content.Ip6 {
			if i >= PRINT_LIMIT {
				if len(content.Ip6)-i == 1 {
					res += fmt.Sprintf("... и %d IP-адрес\n", len(content.Ip6)-i)
				} else if len(content.Ip6)-i > 1 && len(content.Ip6)-i < 5 {
					res += fmt.Sprintf("... и ещё %d IP-адреса\n", len(content.Ip6)-i)
				} else {
					res += fmt.Sprintf("... и ещё %d IP-адресов\n", len(content.Ip6)-i)
				}
				break
			}
			res += fmt.Sprintf("  IP: %s\n", net.IP(ip.Ip6).String())
		}
		for i, sb := range content.Subnet4 {
			if i >= PRINT_LIMIT {
				if len(content.Subnet4)-i == 1 {
					res += fmt.Sprintf("... и %d подсеть\n", len(content.Subnet4)-i)
				} else if len(content.Subnet4)-i > 1 && len(content.Subnet4)-i < 5 {
					res += fmt.Sprintf("... и ещё %d подсети\n", len(content.Subnet4)-i)
				} else {
					res += fmt.Sprintf("... и ещё %d подсетей\n", len(content.Subnet4)-i)
				}
				break
			}
			res += fmt.Sprintf("  Подсеть: %s\n", sb.Subnet4)
		}
		for i, sb := range content.Subnet6 {
			if i >= PRINT_LIMIT {
				if len(content.Subnet6)-i == 1 {
					res += fmt.Sprintf("... и %d подсеть\n", len(content.Subnet6)-i)
				} else if len(content.Subnet6)-i > 1 && len(content.Subnet6)-i < 5 {
					res += fmt.Sprintf("... и ещё %d подсети\n", len(content.Subnet6)-i)
				} else {
					res += fmt.Sprintf("... и ещё %d подсетей\n", len(content.Subnet6)-i)
				}
				break
			}
			res += fmt.Sprintf("  Подсеть: %s\n", sb.Subnet6)
		}
		break
	}
	if mass == "" {
		res = "\n" + res
	} else {
		res = mass + res
	}
	res += printUpToDate(oldest)
	return
}

func constructResult(a []*pb.Content, o TPagination) (res string, pages []TPagination) {
	var mass string
	var oldest int64 = MAX_TIMESTAMP
	var ra []TReason
	if len(a) == 0 {
		return
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].Id < a[j].Id
	})
	ra = make([]TReason, 1)
	ra[0].Id = a[0].Id
	if a[0].Aggr != "" {
		ra[0].Aggr = append(ra[0].Aggr, strings.Split(a[0].Aggr, ",")...)
	}
	if a[0].Ip4 != 0 {
		ra[0].Ip = append(ra[0].Ip, int2Ip4(a[0].Ip4))
	}
	if len(a[0].Ip6) != 0 {
		ra[0].Ip = append(ra[0].Ip, net.IP(a[0].Ip6).String())
	}
	if a[0].Domain != "" {
		ra[0].Domain = append(ra[0].Domain, PrintedDomain(a[0].Domain))
	}
	if a[0].Url != "" {
		ra[0].Url = append(ra[0].Url, PrintedDomain(a[0].Url))
	}
	for i := 0; i < len(a)-1; i++ {
		if a[i].Id == a[i+1].Id {
			if a[i+1].Aggr != "" {
				ra[i].Aggr = append(ra[i].Aggr, strings.Split(a[i+1].Aggr, ",")...)
			}
			if a[i+1].Ip4 != 0 {
				ra[i].Ip = append(ra[i].Ip, int2Ip4(a[i+1].Ip4))
			}
			if len(a[i+1].Ip6) != 0 {
				ra[i].Ip = append(ra[i].Ip, net.IP(a[i+1].Ip6).String())
			}
			if a[i+1].Domain != "" {
				ra[i].Domain = append(ra[i].Domain, a[i+1].Domain)
			}
			if a[i+1].Url != "" {
				ra[i].Url = append(ra[i].Url, a[i+1].Url)
			}
			a = append(a[:i], a[i+1:]...)
			i--
		} else {
			ra = append(ra, TReason{})
			ra[i+1].Id = a[i+1].Id
			if a[i+1].Aggr != "" {
				ra[i+1].Aggr = append(ra[i+1].Aggr, strings.Split(a[i+1].Aggr, ",")...)
			}
			if a[i+1].Ip4 != 0 {
				ra[i+1].Ip = append(ra[i+1].Ip, int2Ip4(a[i+1].Ip4))
			}
			if len(a[i+1].Ip6) != 0 {
				ip6 := net.IP(a[i+1].Ip6)
				ra[i+1].Ip = append(ra[i+1].Ip, ip6.String())
			}
			if a[i+1].Domain != "" {
				ra[i+1].Domain = append(ra[i+1].Domain, a[i+1].Domain)
			}
			if a[i+1].Url != "" {
				ra[i+1].Url = append(ra[i+1].Url, a[i+1].Url)
			}

		}
	}
	sort.Slice(a, func(j, i int) bool {
		if a[i].BlockType == TBLOCK_URL && a[j].BlockType != TBLOCK_URL {
			return true
		} else if a[i].BlockType == TBLOCK_HTTPS &&
			(a[j].BlockType != TBLOCK_URL &&
				a[j].BlockType != TBLOCK_HTTPS) {
			return true
		} else if a[i].BlockType == TBLOCK_DOMAIN &&
			(a[j].BlockType != TBLOCK_URL &&
				a[j].BlockType != TBLOCK_HTTPS &&
				a[j].BlockType != TBLOCK_DOMAIN) {
			return true
		} else if a[i].BlockType == TBLOCK_MASK &&
			(a[j].BlockType != TBLOCK_URL &&
				a[j].BlockType != TBLOCK_HTTPS &&
				a[j].BlockType != TBLOCK_DOMAIN &&
				a[j].BlockType != TBLOCK_MASK) {
			return true
		} else {
			return false
		}
	})
	offset := 0
	if o.Tag == OFFSET_CONTENT {
		if len(a) <= PRINT_LIMIT {
			offset = 0
		} else if o.Count > len(a)-(len(a)%PRINT_LIMIT) {
			offset = len(a) - (len(a) % PRINT_LIMIT)
		} else {
			offset = o.Count
		}
	}
	var cnt, cbu, cbh, cbd, cbm, cbi int
	for i, packet := range a {
		if o.Tag == OFFSET_CONTENT && i < offset {
			continue
		}
		content := TContent{}
		err := json.Unmarshal(packet.Pack, &content)
		if err != nil {
			Error.Printf("Упс!!! %s\n", err)
			continue
		}
		if packet.RegistryUpdateTime < oldest {
			oldest = packet.RegistryUpdateTime
		}
		var req TReason
		for _, req = range ra {
			if req.Id == packet.Id {
				break
			}
		}
		if len(req.Aggr) != 0 {
			if packet.BlockType == TBLOCK_IP {
				mass = "\U0001f4a5\U0001f4a5\U0001f4a5 Ресурс под «ковровой» блокировкой!!\n\n"
			}
		}
		if cnt < PRINT_LIMIT {
			bt := ""
			switch packet.BlockType {
			case TBLOCK_URL:
				bt = "\U000026d4 "
				cbu++
			case TBLOCK_HTTPS:
				bt = "\U0001f4db "
				cbh++
			case TBLOCK_DOMAIN:
				bt = "\U0001f6ab "
				cbd++
			case TBLOCK_MASK:
				bt = "\U0001f506 "
				cbm++
			case TBLOCK_IP:
				bt = "\u274c "
				cbi++
			}
			dcs := fmt.Sprintf("%s %s %s", content.Decision.Org, content.Decision.Number, content.Decision.Date)
			res += fmt.Sprintf("%s /n\\_%d %s\n", bt, content.Id, dcs)
			res += fmt.Sprintf("\u2022 %s\n", constructBasis(&content))
			if len(req.Aggr) != 0 {
				for _, nw := range req.Aggr {
					res += fmt.Sprintf("    _как подсеть_ %s\n", nw)
				}
			}
			if len(req.Ip) != 0 {
				for _, ip := range req.Ip {
					res += fmt.Sprintf("    _как ip_ %s\n", ip)
				}
			}
			if len(req.Domain) != 0 {
				for _, domain := range req.Domain {
					res += fmt.Sprintf("    _как domain_ %s\n", Sanitize(PrintedDomain(domain)))
				}
			}
			if len(req.Url) != 0 {
				for _, u := range req.Url {
					res += fmt.Sprintf("    _как url_ %s\n", Sanitize(PrintedDomain(u)))
				}
			}
			res += "\n"
		}
		cnt++
	}
	if mass != "" {
		res = mass + res
	}
	if cnt > PRINT_LIMIT {
		pages = append(pages, TPagination{OFFSET_CONTENT, len(a)})
		//rest := cnt - PRINT_LIMIT
		res += fmt.Sprintf("\u2194 результаты с %d по %d из %d\n", offset+1, offset+1+PRINT_LIMIT, len(a))
		/*if cbu > 0 && cbu < rest {
			res += fmt.Sprintf(" url=%d", cbu)
		} else if cbu > 0 {
			res += fmt.Sprintf(" url=%d", rest)
		}
		if cbh > 0 && cbu+cbh < rest {
			res += fmt.Sprintf(" https=%d", cbu)
		} else if cbh > 0 {
			res += fmt.Sprintf(" https=%d", rest-cbu)
		}
		if cbd > 0 && cbd+cbu+cbh < rest {
			res += fmt.Sprintf(" domain=%d", cbu)
		} else if cbd > 0 {
			res += fmt.Sprintf(" domain=%d", rest-cbh-cbu)
		}
		if cbm > 0 && cbm+cbd+cbu+cbh < rest {
			res += fmt.Sprintf(" wildcard=%d", cbu)
		} else if cbm > 0 {
			res += fmt.Sprintf(" wildcard=%d", rest-cbd-cbh-cbu)
		}
		if cbi > 0 && cbm+cbd+cbu+cbh < rest {
			res += fmt.Sprintf(" ip=%d", rest-cbm-cbd-cbh-cbu)
		}*/
		res += "\n"
	}
	var abt []string
	if cbu > 0 {
		abt = append(abt, "url: \U000026d4")
	}
	if cbh > 0 {
		abt = append(abt, "https: \U0001f4db")
	}
	if cbd > 0 {
		abt = append(abt, "domain: \U0001f6ab")
	}
	if cbm > 0 {
		abt = append(abt, "wildcard: \U0001f506")
	}
	if cbi > 0 {
		abt = append(abt, "ip: \u274c")
	}
	res += "*типы блокировки:* " + strings.Join(abt, " | ")
	res += printUpToDate(oldest)
	return
}
