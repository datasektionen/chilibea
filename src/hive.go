package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

type HiveGroup struct {
	Name   string `json:"group_name"`
	Id     string `json:"group_id"`
	Domain string `json:"group_domain"`
	Tag    string `json:"tag_content"`
}

func getHiveGroup() ([]Group, error) {
	url := fmt.Sprintf("%s/tagged/internal/groups", os.Getenv("HIVE_URL"))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Failed to create request:", err)
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("HIVE_TOKEN")))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to make request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Unexpected status code:", "status", resp.StatusCode)
		return nil, err
	}

	var groups []HiveGroup
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		slog.Error("Failed to decode response:", err)
		return nil, err
	}

	var result []Group
	for _, g := range groups {
		members, err := getMembersInGroup(g, client)
		if err != nil {
			slog.Error("Failed to get members for group:", "group", g.Name, "error", err)
			continue
		}
		result = append(result, Group{
			Name:    g.Name,
			Desc:    g.Id,
			Email:   fmt.Sprintf("%s@%s", g.Id, g.Domain),
			Members: members,
		})
	}

	return result, nil
}

type SsoUser struct {
	FirstName  string `json:"firstName"`
	FamilyName string `json:"familyName"`
	Email      string `json:"email"`
	Picture    string `json:"picture"`
	YearTag    string `json:"yearTag"`
}

func getMembersInGroup(group HiveGroup, client *http.Client) ([]Member, error) {
	url := fmt.Sprintf("%s/group/%s/%s/members", os.Getenv("HIVE_URL"), group.Domain, group.Id)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Failed to create request:", err)
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("HIVE_TOKEN")))
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to make request:", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Unexpected status code:", "status", resp.StatusCode)
		return nil, err
	}

	var members []string
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		slog.Error("Failed to decode response:", err)
		return nil, err
	}

	url = fmt.Sprintf("%s/api/users?format=array&picture=thumbnail", os.Getenv("SSO_URL"))
	for _, m := range members {
		url += fmt.Sprintf("&u=%s", m)
	}
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Failed to create request:", err)
		return nil, err
	}
	resp, err = client.Do(req)
	if err != nil {
		slog.Error("Failed to make request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Unexpected status code:", "status", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var users []SsoUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		slog.Error("Failed to decode response:", err)
		return nil, err
	}

	var result []Member
	for _, u := range users {
		result = append(result, Member{
			Name:   fmt.Sprintf("%s %s", u.FirstName, u.FamilyName),
			Email:  u.Email,
			ImgUrl: u.Picture,
		})
	}

	return result, nil
}
