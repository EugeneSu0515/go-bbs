package bbs

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// logins.bad 有兩種，一個在BBSHOME，一個在User下面
// https://github.com/ptt/pttbbs/blob/master/include/common.h#L56
// https://github.com/ptt/pttbbs/blob/master/common/bbs/passwd.c#L255
//
// BBSHOME/logins.bad: 這個檔裡有每個 user的login attempt且包含成功與失敗。第一個字元若是"-"代表失敗。
//
//  test03      [01/01/2021 10:11:45 Fri] ?@172.22.0.1
//  test04      [01/01/2021 10:13:35 Fri] ?@172.22.0.1
//  test05      [01/01/2021 10:13:45 Fri] ?@172.22.0.1
//  SYSOP       [01/01/2021 10:13:53 Fri] ?@172.22.0.1
//  test06      [01/01/2021 10:14:38 Fri] ?@172.22.0.1
//  SYSOP       [01/01/2021 10:14:46 Fri] ?@172.22.0.1
// -test01      [01/01/2021 10:15:16 Fri] ?@172.22.0.1
// -test02      [01/01/2021 10:15:19 Fri] ?@172.22.0.1
// -test03      [01/01/2021 10:15:22 Fri] ?@172.22.0.1
//  test04      [01/01/2021 10:15:38 Fri] ?@172.22.0.1
//
// BBSHOME/home/<x>/<user>/logins.bad: 這個檔裡只有該user的 失敗 login attempt
//
// ╰─➤  cat home/T/test01/logins.bad
// [01/01/2021 10:15:16 Fri] 172.22.0.1
//
// 目前想法是用同一個struct來parse這2種logins.bad
//
// type LoginAttempt struct {
// 	Success        bool
// 	UserId         string
// 	LoginStartTime time.Time
// 	FromHost       string
// }
// For BBSHOME/logins.bad ，這個檔裡四個field都有，所以沒問題。
// 但在user/logins.bad，缺少 UserId ，所以parse出來的struct就沒有 UserId，需要caller assign

const (
	// UserIdLength is fixed to 12
	UserIdLength = 12
	// FromHostPrefix is a prefix affixed to ip only in BBSHOME/logins.bad
	fromHostPrefix             = "?@"
	loginStartTimeFormatString = "[01/02/2006 15:04:05 Mon]"
)

var (
	InvalidLoginsBadFormat = errors.New("Invalid logins.bad line format")
)

// LoginAttempt represents an entry in logins.bad file to indicate a successful or failed login
// attempt for a UserId. Note that UserId could be empty if the logins.bad is under user dir.
type LoginAttempt struct {
	Success        bool
	UserId         string
	LoginStartTime time.Time
	FromHost       string
}

// OpenBadLoginFile opens logins.bad file and returns a slice of LoginAttempt.
// Note that depending on different format of logins.bad as descirbed above, each LoginAttempt
// might not have LoginAttempt.UserId field
func OpenBadLoginFile(filename string) ([]*LoginAttempt, error) {
	file, err := os.Open(filename)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	var ret []*LoginAttempt

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		a := &LoginAttempt{}
		err = a.UnmarshalText(line)
		if err != nil {
			return nil, err
		}
		ret = append(ret, a)
	}
	return ret, nil
}

// UnmarshalText implements encoding.TextUnmarshaler to unmarshal text to the receiver
func (l *LoginAttempt) UnmarshalText(text []byte) error {
	str := string(text)

	idx := 0 // current index of str
	// Handle Success and UserId
	switch str[idx] {
	case ' ':
		idx += 1
		l.Success = true
		// Next 12 is UserId
		l.UserId = str[idx : idx+UserIdLength]
		idx += UserIdLength
	case '-':
		idx += 1
		l.Success = false
		l.UserId = str[idx : idx+UserIdLength]
		idx += UserIdLength
	case '[':
		// This indicates this line has no Success and UserId, set Success to false
		l.Success = false
		l.UserId = ""
	default:
		return InvalidLoginsBadFormat
	}
	l.UserId = strings.TrimSpace(l.UserId)
	// Now idx points to the start of time
	// TODO: do we need to consider timezone? This Parse returns UTC
	t, err := time.Parse(loginStartTimeFormatString, str[idx:idx+len(loginStartTimeFormatString)])
	if err != nil {
		return err
	}
	l.LoginStartTime = t
	idx += len(loginStartTimeFormatString)

	l.FromHost = strings.TrimLeft(str[idx+1:], fromHostPrefix)
	return nil
}

// MarshalText implements encoding.TextMarshaler to marshal receiver to text
func (l *LoginAttempt) MarshalText() ([]byte, error) {
	var sb strings.Builder
	if l.IsUnderBbsHome() {
		if l.Success {
			sb.WriteRune(' ')
		} else {
			sb.WriteRune('-')
		}
		// Right padding UserId
		sb.WriteString(fmt.Sprintf("%-*s", UserIdLength, l.UserId))
	}
	// time
	formatted := ""
	// TODO: consider timezone?
	formatted = l.LoginStartTime.Format(loginStartTimeFormatString)
	sb.WriteString(formatted)
	sb.WriteRune(' ')
	// ip
	if l.IsUnderBbsHome() {
		sb.WriteString(fromHostPrefix)
	}
	sb.WriteString(l.FromHost)

	return []byte(sb.String()), nil
}

// IsUnderBbsHome return true if this LoginAttempt was read from logins.bad from under BBSHOME.
// The difference between logins.bad between under BBSHOME and under User Dir is whether it contains
// UserId
func (l *LoginAttempt) IsUnderBbsHome() bool {
	return len(l.UserId) > 1
}
