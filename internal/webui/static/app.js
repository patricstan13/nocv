(function () {
  "use strict";

  const inspector = document.getElementById("inspector");
  const networkElement = document.getElementById("network");

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
    const nodes = new vis.DataSet(data.nodes.map((node) => ({
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
