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

  function symbolItem(value) {
    return element("li", value.id || value, "symbol");
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

  function dependencyItem(value) {
    return element("li", value.from + " → " + value.to, "symbol");
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
      parts.push(...section("Types", detail.types, symbolItem));
      parts.push(...section("Functions", detail.functions, symbolItem));
      parts.push(...section("Semantic dependencies", detail.dependencies, dependencyItem));
      parts.push(...section("Semantic dependents", detail.dependents, dependencyItem));
      parts.push(...section("Imports", detail.imports, relationshipItem));
      parts.push(...section("Imported by", detail.importers, relationshipItem));
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
        element("h2", from + " → " + to),
      ];
      parts.push(...section("Type relationships", result.typeDependencies, (dependency) => {
        const item = element("li", undefined, "relationship");
        item.appendChild(element("div", dependency.from + " → " + dependency.to, "symbol"));
        const evidence = element("ul");
        dependency.evidence.forEach((value) => evidence.appendChild(relationshipItem(value)));
        item.appendChild(evidence);
        return item;
      }));
      parts.push(...section("Exact-only relationships", result.exactOnly, relationshipItem));
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
      from: edge.from,
      to: edge.to,
      arrows: { to: { enabled: true, scaleFactor: 0.7 } },
    })));
    const network = new vis.Network(networkElement, { nodes, edges }, {
      layout: { randomSeed: 240519, improvedLayout: true },
      physics: {
        enabled: true,
        stabilization: { enabled: true, iterations: 600, fit: true },
        barnesHut: { gravitationalConstant: -4200, springLength: 170, springConstant: 0.035 },
      },
      interaction: { dragNodes: true, dragView: true, hover: true, multiselect: false, zoomView: true },
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

    network.on("click", (params) => {
      if (params.nodes.length > 0) {
        showPackage(params.nodes[0]);
      } else if (params.edges.length > 0) {
        const selected = edges.get(params.edges[0]);
        showDependency(selected.from, selected.to);
      }
    });
  }).catch(showError);
}());
