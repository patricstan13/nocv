(function () {
  "use strict";

  const inspector = document.getElementById("inspector");
  const networkElement = document.getElementById("network");
  const searchInput = document.getElementById("package-search-input");
  const searchResults = document.getElementById("package-search-results");
  const SEARCH_RESULT_LIMIT = 10;

  function element(tag, text, className) {
    const item = document.createElement(tag);
    if (text !== undefined) item.textContent = text;
    if (className) item.className = className;
    return item;
  }

  async function getJSON(url) {
    const response = await fetch(url, { headers: { Accept: "application/json" } });
    let body;
    try {
      body = await response.json();
    } catch (_) {
      throw new Error("The server returned an unreadable response.");
    }
    if (!response.ok) throw new Error(body.error || "Request failed.");
    return body;
  }

  function replaceInspector(parts) {
    inspector.replaceChildren(...parts);
    inspector.scrollTop = 0;
  }

  function section(title, values, render) {
    const parts = [element("h3", title)];
    if (!values || values.length === 0) {
      parts.push(element("p", "None", "muted"));
      return parts;
    }
    const list = element("ul");
    values.forEach((value) => list.appendChild(render(value)));
    parts.push(list);
    return parts;
  }

  function relationshipItem(value) {
    const item = element("li", undefined, "relationship");
    const summary = element("div");
    summary.appendChild(element("strong", value.kind));
    summary.appendChild(document.createTextNode("  " + value.from + " → " + value.to));
    item.appendChild(summary);
    const evidence = value.evidence || [];
    if (evidence.length > 0) {
      item.appendChild(element("span", evidence.map(formatLocation).join(" · "), "evidence"));
    }
    return item;
  }

  function dependencyTargetItem(value) {
    return element("li", value.to, "symbol");
  }

  function dependencySourceItem(value) {
    return element("li", value.from, "symbol");
  }

  function contentsSummary(detail) {
    const list = element("ul", undefined, "contents-summary");
    list.appendChild(element("li", (detail.types || []).length + " types"));
    list.appendChild(element("li", (detail.functions || []).length + " functions"));
    return [element("h3", "Contents"), list];
  }

  function collapsibleRelationships(values, threshold, showLabel) {
    const container = element("div", undefined, "evidence-group");
    const list = element("ul");
    values.forEach((value) => list.appendChild(relationshipItem(value)));
    if (values.length <= threshold) {
      container.appendChild(list);
      return container;
    }

    list.hidden = true;
    const toggle = element("button", showLabel, "evidence-toggle");
    toggle.type = "button";
    toggle.setAttribute("aria-expanded", "false");
    toggle.addEventListener("click", () => {
      const expanded = list.hidden;
      list.hidden = !expanded;
      toggle.textContent = expanded ? "Hide evidence" : showLabel;
      toggle.setAttribute("aria-expanded", String(expanded));
    });
    container.append(toggle, list);
    return container;
  }

  function formatLocation(value) {
    if (!value.file) return "unknown location";
    return value.file + (value.offset ? ":" + value.offset : "");
  }

  function compareText(left, right) {
    if (left < right) return -1;
    if (left > right) return 1;
    return 0;
  }

  function searchPackages(packageNodes, query) {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return { matches: [], total: 0 };

    const ranked = packageNodes.map((node) => {
      const label = node.label.toLowerCase();
      const ref = node.id.toLowerCase();
      let rank = 4;
      if (label === normalized) rank = 0;
      else if (label.startsWith(normalized)) rank = 1;
      else if (label.includes(normalized)) rank = 2;
      else if (ref.includes(normalized)) rank = 3;
      return { node, rank, label, ref };
    }).filter((candidate) => candidate.rank < 4);

    ranked.sort((left, right) => left.rank - right.rank || compareText(left.label, right.label) || compareText(left.ref, right.ref));
    return {
      matches: ranked.slice(0, SEARCH_RESULT_LIMIT).map((candidate) => candidate.node),
      total: ranked.length,
    };
  }

  function setupPackageSearch(packageNodes, network) {
    let visibleResults = [];
    let activeIndex = -1;

    function closeSearchResults() {
      searchResults.hidden = true;
      searchInput.setAttribute("aria-expanded", "false");
      activeIndex = -1;
    }

    function setActiveIndex(index) {
      if (visibleResults.length === 0) return;
      activeIndex = (index + visibleResults.length) % visibleResults.length;
      Array.from(searchResults.querySelectorAll(".package-search-result")).forEach((item, itemIndex) => {
        const active = itemIndex === activeIndex;
        item.classList.toggle("is-active", active);
        item.setAttribute("aria-selected", String(active));
        if (active) item.scrollIntoView({ block: "nearest" });
      });
    }

    function selectSearchResult(node) {
      network.selectNodes([node.id], true);
      network.focus(node.id, {
        scale: 0.75,
        animation: { duration: 450, easingFunction: "easeInOutQuad" },
      });
      showPackage(node.id);
      closeSearchResults();
    }

    function renderSearchResults() {
      const query = searchInput.value;
      if (!query.trim()) {
        visibleResults = [];
        searchResults.replaceChildren();
        closeSearchResults();
        return;
      }

      const result = searchPackages(packageNodes, query);
      visibleResults = result.matches;
      const parts = [];
      visibleResults.forEach((node, index) => {
        const item = element("button", undefined, "package-search-result");
        item.type = "button";
        item.setAttribute("role", "option");
        item.setAttribute("aria-selected", "false");
        item.appendChild(element("span", node.label, "package-search-label"));
        item.appendChild(element("span", node.id, "package-search-ref"));
        item.addEventListener("mouseenter", () => setActiveIndex(index));
        item.addEventListener("click", () => selectSearchResult(node));
        parts.push(item);
      });
      if (result.total === 0) {
        parts.push(element("div", "No matching packages", "package-search-status"));
      } else if (result.total > visibleResults.length) {
        parts.push(element("div", "+ " + (result.total - visibleResults.length) + " more matches", "package-search-status"));
      }
      searchResults.replaceChildren(...parts);
      searchResults.hidden = false;
      searchInput.setAttribute("aria-expanded", "true");
      activeIndex = visibleResults.length > 0 ? 0 : -1;
      if (activeIndex >= 0) setActiveIndex(activeIndex);
    }

    searchInput.addEventListener("input", renderSearchResults);
    searchInput.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeSearchResults();
      } else if (event.key === "ArrowDown" && visibleResults.length > 0) {
        event.preventDefault();
        setActiveIndex(activeIndex + 1);
      } else if (event.key === "ArrowUp" && visibleResults.length > 0) {
        event.preventDefault();
        setActiveIndex(activeIndex - 1);
      } else if (event.key === "Enter" && activeIndex >= 0) {
        event.preventDefault();
        selectSearchResult(visibleResults[activeIndex]);
      }
    });
  }

  async function showPackage(id) {
    try {
      const result = await getJSON("/api/node?id=" + encodeURIComponent(id));
      const detail = result.package;
      const parts = [
        element("p", "Package", "eyebrow"),
        element("h2", result.node.id),
        element("span", result.node.kind, "kind"),
        element("h3", "Documentation"),
        element("p", result.node.documentation || "No package documentation.", result.node.documentation ? "documentation" : "muted"),
      ];
      parts.push(...section("Dependencies", detail.dependencies, dependencyTargetItem));
      parts.push(...section("Dependents", detail.dependents, dependencySourceItem));
      parts.push(...section("Imports", detail.imports, dependencyTargetItem));
      parts.push(...section("Imported by", detail.importers, dependencySourceItem));
      parts.push(...contentsSummary(detail));
      replaceInspector(parts);
    } catch (error) {
      showError(error);
    }
  }

  async function showDependency(from, to) {
    try {
      const url = "/api/package-dependency?from=" + encodeURIComponent(from) + "&to=" + encodeURIComponent(to);
      const result = await getJSON(url);
      const parts = [
        element("p", "Semantic dependency", "eyebrow"),
        element("h2", from + " depends on " + to),
      ];
      parts.push(...section("Type relationships", result.typeDependencies, (dependency) => {
        const item = element("li", undefined, "relationship");
        const facts = dependency.evidence || [];
        item.appendChild(element("div", dependency.from + " → " + dependency.to + " (" + facts.length + " facts)", "symbol"));
        item.appendChild(collapsibleRelationships(facts, 2, "Show evidence"));
        return item;
      }));
      const exact = result.exactOnly || [];
      parts.push(element("h3", "Exact-only relationships (" + exact.length + ")"));
      if (exact.length === 0) {
        parts.push(element("p", "None", "muted"));
      } else {
        parts.push(collapsibleRelationships(exact, 5, "Show relationships"));
      }
      replaceInspector(parts);
    } catch (error) {
      showError(error);
    }
  }

  function showError(error) {
    replaceInspector([
      element("p", "Inspector", "eyebrow"),
      element("h2", "Unable to inspect selection"),
      element("p", error.message, "error"),
    ]);
  }

  getJSON("/api/packages").then((data) => {
    const packageNodes = data.nodes.slice();
    const nodes = new vis.DataSet(packageNodes.map((node) => ({
      id: node.id,
      label: node.label,
      title: node.id,
      shape: "box",
      margin: 12,
    })));
    const edges = new vis.DataSet(data.edges.map((edge) => ({
      id: edge.id,
      from: edge.to,
      to: edge.from,
      semanticFrom: edge.from,
      semanticTo: edge.to,
      arrows: { to: { enabled: true, scaleFactor: 0.7 } },
    })));
    const network = new vis.Network(networkElement, { nodes, edges }, {
      layout: { randomSeed: 240519, improvedLayout: true },
      physics: {
        enabled: true,
        stabilization: { enabled: true, iterations: 600, fit: true },
        barnesHut: { gravitationalConstant: -4200, springLength: 170, springConstant: 0.035 },
      },
      interaction: { dragNodes: true, dragView: true, hover: true, hoverConnectedEdges: true, multiselect: false, selectConnectedEdges: true, zoomView: true },
      nodes: {
        borderWidth: 1,
        color: { background: "#ffffff", border: "#64748b", highlight: { background: "#cffafe", border: "#0891b2" } },
        font: { color: "#172033", face: "system-ui", size: 14 },
      },
      edges: {
        color: { color: "#38bdf8", highlight: "#0369a1", hover: "#0284c7" },
        width: 1.5,
        selectionWidth: 2,
        smooth: { enabled: true, type: "dynamic" },
      },
    });

    let physicsFrozen = false;
    let freezeFallback;
    const freezePhysics = () => {
      if (physicsFrozen) return;
      physicsFrozen = true;
      window.clearTimeout(freezeFallback);
      network.stopSimulation();
      network.setOptions({ physics: false });
    };
    network.once("stabilizationIterationsDone", freezePhysics);
    network.once("stabilized", freezePhysics);
    freezeFallback = window.setTimeout(freezePhysics, 8000);
    setupPackageSearch(packageNodes, network);

    network.on("click", (params) => {
      if (params.nodes.length > 0) {
        showPackage(params.nodes[0]);
      } else if (params.edges.length > 0) {
        const selected = edges.get(params.edges[0]);
        showDependency(selected.semanticFrom, selected.semanticTo);
      }
    });
  }).catch(showError);
}());
