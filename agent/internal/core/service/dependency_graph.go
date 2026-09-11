package service

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// PublishedPort representa un enlace de puerto publicado en el host.
type PublishedPort struct {
	HostIP        string
	HostPort      int
	ContainerPort int
	Protocol      string
}

// IsLoopback verifica si el bind está en localhost/127.0.0.1.
func (p PublishedPort) IsLoopback() bool {
	ip := strings.TrimSpace(p.HostIP)
	return ip == "127.0.0.1" || strings.HasPrefix(ip, "127.") || ip == "localhost" || ip == "::1"
}

// Glyph retorna la etiqueta de exposición informativa sin falsas alarmas (R3).
func (p PublishedPort) Glyph() string {
	if p.IsLoopback() {
		return "[OK] loopback"
	}
	return "[||] expuesto"
}

// DependencyGraph mapea relaciones de dependencias, radio de impacto y propiedad de puertos entre contenedores.
type DependencyGraph struct {
	containers    map[string]domain.ContainerMetric
	dependsOn     map[string][]string // ID y Name -> [nombres de dependencias]
	dependents    map[string][]string // ID y Name -> [nombres de dependientes]
	portOwners    map[int]string      // hostPort -> containerName running
	sharedPeers   map[string][]string // ID y Name -> [nombres de contenedores en mismas redes]
	networksOf    map[string][]string // ID y Name -> redes
}

// NewDependencyGraph construye el grafo determinístico a partir de la telemetría actual de la flota.
func NewDependencyGraph(containers []domain.ContainerMetric) *DependencyGraph {
	g := &DependencyGraph{
		containers:  make(map[string]domain.ContainerMetric),
		dependsOn:   make(map[string][]string),
		dependents:  make(map[string][]string),
		portOwners:  make(map[int]string),
		sharedPeers: make(map[string][]string),
		networksOf:  make(map[string][]string),
	}

	for _, c := range containers {
		g.containers[c.ID] = c
		g.containers[c.Name] = c
		g.networksOf[c.ID] = c.Networks
		g.networksOf[c.Name] = c.Networks

		// Mapa de propiedad de puertos: solo contenedores en estado running/Up
		statusLower := strings.ToLower(c.Status)
		if strings.HasPrefix(statusLower, "up") || statusLower == "running" {
			ports := g.GetPublishedPorts(c)
			for _, p := range ports {
				if p.HostPort > 0 {
					g.portOwners[p.HostPort] = c.Name
				}
			}
		}
	}

	// 1. Identificar redes compartidas entre contenedores
	for i, c := range containers {
		var peers []string
		for j, h := range containers {
			if i == j || c.ID == h.ID || c.Name == h.Name {
				continue
			}
			if haveSharedNetwork(c.Networks, h.Networks) {
				peers = append(peers, h.Name)
			}
		}
		sort.Strings(peers)
		g.sharedPeers[c.ID] = peers
		g.sharedPeers[c.Name] = peers
	}

	// 2. Inferir dependencias a partir de los valores de EnvVars
	for _, c := range containers {
		// Construir identificadores válidos de hermanos con los que comparte red
		peerIdentMap := make(map[string]string) // ident_lowercase -> targetContainerName
		for _, h := range containers {
			if c.ID == h.ID || c.Name == h.Name {
				continue
			}
			if !haveSharedNetwork(c.Networks, h.Networks) {
				continue
			}
			// Registrar nombre del contenedor hermano
			cleanName := strings.TrimPrefix(h.Name, "/")
			peerIdentMap[strings.ToLower(cleanName)] = cleanName
			peerIdentMap[strings.ToLower(h.ID)] = cleanName

			// Registrar alias de red del hermano
			for _, alias := range h.NetworkAliases {
				cleanAlias := strings.TrimSpace(alias)
				if cleanAlias != "" {
					peerIdentMap[strings.ToLower(cleanAlias)] = cleanName
				}
			}
		}

		// Recorrer los valores de EnvVars de C
		depsSet := make(map[string]bool)
		for _, env := range c.EnvVars {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) < 2 {
				continue
			}
			val := parts[1]
			tokens := ExtractHostTokens(val)
			for _, token := range tokens {
				if targetName, ok := peerIdentMap[strings.ToLower(token)]; ok {
					if targetName != c.Name && targetName != strings.TrimPrefix(c.Name, "/") {
						depsSet[targetName] = true
					}
				}
			}
		}

		var depsList []string
		for target := range depsSet {
			depsList = append(depsList, target)
		}
		sort.Strings(depsList)

		if len(depsList) > 0 {
			g.dependsOn[c.ID] = depsList
			g.dependsOn[c.Name] = depsList

			// Invertir para "dependen de mi"
			for _, target := range depsList {
				g.dependents[target] = append(g.dependents[target], c.Name)
			}
		}
	}

	// Deduplicar y ordenar dependientes
	for target, list := range g.dependents {
		unique := make(map[string]bool)
		var cleaned []string
		for _, name := range list {
			if !unique[name] {
				unique[name] = true
				cleaned = append(cleaned, name)
			}
		}
		sort.Strings(cleaned)
		g.dependents[target] = cleaned
	}

	return g
}

// haveSharedNetwork comprueba si dos listas de redes tienen al menos una red compartida.
func haveSharedNetwork(netsA, netsB []string) bool {
	for _, a := range netsA {
		if a == "" {
			continue
		}
		for _, b := range netsB {
			if a == b {
				return true
			}
		}
	}
	return false
}

// ExtractHostTokens extrae candidatos de hostname de un valor de variable de entorno.
func ExtractHostTokens(envValue string) []string {
	var tokens []string
	seen := make(map[string]bool)

	add := func(t string) {
		t = strings.Trim(t, " \t\r\n'\"`")
		if t == "" {
			return
		}
		// Ignorar números puros (ej: puertos) o direcciones IP
		if _, err := strconv.Atoi(t); err == nil {
			return
		}
		if net.ParseIP(t) != nil {
			return
		}
		lower := strings.ToLower(t)
		if lower == "true" || lower == "false" || lower == "yes" || lower == "no" || lower == "null" || lower == "none" {
			return
		}
		if !seen[lower] {
			seen[lower] = true
			tokens = append(tokens, t)
		}
	}

	// 1. Patrones scheme://[user:pass@]host[:port][/path] (ej: postgres://..., redis://..., http://...)
	uriRegex := regexp.MustCompile(`[a-zA-Z0-9+.-]+://(?:[^:@/]+(?::[^@/]*)?@)?([a-zA-Z0-9_.-]+)(?::[0-9]+)?`)
	matches := uriRegex.FindAllStringSubmatch(envValue, -1)
	for _, m := range matches {
		if len(m) > 1 {
			add(m[1])
		}
	}

	// 2. Parámetros host=valor o server=valor (ej: libpq connection strings)
	hostParamRegex := regexp.MustCompile(`\b(?:host|server|hostname|endpoint)=([a-zA-Z0-9_.-]+)`)
	matchesParam := hostParamRegex.FindAllStringSubmatch(envValue, -1)
	for _, m := range matchesParam {
		if len(m) > 1 {
			add(m[1])
		}
	}

	// 3. Patrón host:port (ej: db:5432, redis:6379)
	hostPortRegex := regexp.MustCompile(`\b([a-zA-Z0-9_.-]+):[0-9]{1,5}\b`)
	matchesHP := hostPortRegex.FindAllStringSubmatch(envValue, -1)
	for _, m := range matchesHP {
		if len(m) > 1 {
			add(m[1])
		}
	}

	// 4. Si el valor completo o fragmentos separados por comas son hostnames simples (ej: DB_HOST=db)
	subparts := strings.FieldsFunc(envValue, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	simpleIdentRegex := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	for _, sub := range subparts {
		clean := strings.Trim(sub, " \t\r\n'\"`")
		if simpleIdentRegex.MatchString(clean) {
			add(clean)
		}
	}

	return tokens
}

// ParsePublishedPort analiza un string de puerto hacia PublishedPort.
func ParsePublishedPort(portStr string) (PublishedPort, bool) {
	s := strings.TrimSpace(portStr)
	if s == "" {
		return PublishedPort{}, false
	}

	if strings.Contains(s, "->") {
		parts := strings.Split(s, "->")
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])

		// Caso A: containerPort -> hostIP:hostPort (notación V5)
		if strings.Contains(right, ":") && (strings.Contains(right, ".") || strings.Contains(right, "::") || strings.HasPrefix(right, "localhost")) {
			cPort, proto := parsePortProto(left)
			idx := strings.LastIndex(right, ":")
			hIP := right[:idx]
			hPort, _ := strconv.Atoi(right[idx+1:])
			return PublishedPort{
				HostIP:        hIP,
				HostPort:      hPort,
				ContainerPort: cPort,
				Protocol:      proto,
			}, true
		}

		// Caso B: hostIP:hostPort -> containerPort (notación Docker estándar)
		if strings.Contains(left, ":") {
			idx := strings.LastIndex(left, ":")
			hIP := left[:idx]
			hPort, _ := strconv.Atoi(left[idx+1:])
			cPort, proto := parsePortProto(right)
			return PublishedPort{
				HostIP:        hIP,
				HostPort:      hPort,
				ContainerPort: cPort,
				Protocol:      proto,
			}, true
		}

		// Caso C: hostPort -> containerPort sin IP explícita
		hPort, err := strconv.Atoi(left)
		if err == nil {
			cPort, proto := parsePortProto(right)
			return PublishedPort{
				HostIP:        "0.0.0.0",
				HostPort:      hPort,
				ContainerPort: cPort,
				Protocol:      proto,
			}, true
		}
	}

	return PublishedPort{}, false
}

func parsePortProto(s string) (int, string) {
	s = strings.TrimSpace(s)
	proto := "tcp"
	if strings.Contains(s, "/") {
		parts := strings.Split(s, "/")
		s = parts[0]
		if len(parts) > 1 && parts[1] != "" {
			proto = parts[1]
		}
	}
	p, _ := strconv.Atoi(s)
	return p, proto
}

// GetPublishedPorts extrae y ordena ascendentemente por hostPort los puertos publicados.
func (g *DependencyGraph) GetPublishedPorts(c domain.ContainerMetric) []PublishedPort {
	var result []PublishedPort
	for _, pStr := range c.Ports {
		if p, ok := ParsePublishedPort(pStr); ok && p.HostPort > 0 {
			result = append(result, p)
		}
	}
	// Orden ascendente por HostPort
	sort.Slice(result, func(i, j int) bool {
		return result[i].HostPort < result[j].HostPort
	})
	return result
}

// GetDependencies devuelve la lista ordenada de nombres de contenedores de los que depende C.
func (g *DependencyGraph) GetDependencies(c domain.ContainerMetric) []string {
	if deps, ok := g.dependsOn[c.Name]; ok {
		return deps
	}
	if deps, ok := g.dependsOn[c.ID]; ok {
		return deps
	}
	return nil
}

// GetDependents devuelve la lista ordenada de nombres de contenedores que dependen de C.
func (g *DependencyGraph) GetDependents(c domain.ContainerMetric) []string {
	if deps, ok := g.dependents[c.Name]; ok {
		return deps
	}
	if deps, ok := g.dependents[c.ID]; ok {
		return deps
	}
	return nil
}

// GetImpactRadius calcula cuántos contenedores comparten redes con C (excluyéndose a sí mismo).
func (g *DependencyGraph) GetImpactRadius(c domain.ContainerMetric) (int, string) {
	peers := g.sharedPeers[c.Name]
	if len(peers) == 0 {
		peers = g.sharedPeers[c.ID]
	}
	count := len(peers)
	netStr := strings.Join(c.Networks, ", ")
	if netStr == "" {
		netStr = "sin red"
	}

	if count == 1 {
		return count, fmt.Sprintf("1 contenedor en %s", netStr)
	}
	return count, fmt.Sprintf("%d contenedores en %s", count, netStr)
}

// GetPortConflicts identifica si algún puerto publicado por C está retenido por otro contenedor running.
func (g *DependencyGraph) GetPortConflicts(c domain.ContainerMetric) map[int]string {
	conflicts := make(map[int]string)
	ports := g.GetPublishedPorts(c)
	for _, p := range ports {
		if owner, ok := g.portOwners[p.HostPort]; ok {
			if owner != c.Name && owner != strings.TrimPrefix(c.Name, "/") {
				conflicts[p.HostPort] = owner
			}
		}
	}
	return conflicts
}

// GetPortOwners devuelve el mapa global de propiedad de puertos (puerto host -> contenedor running).
func (g *DependencyGraph) GetPortOwners() map[int]string {
	return g.portOwners
}
