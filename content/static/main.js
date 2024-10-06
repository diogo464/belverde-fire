const DEFAULT_ZOOM = 14;
const DEFAULT_COORDINATES = [38.59104623572979, -9.130882470026634];

const ELEM_ID_MAP = "map";
const ELEM_ID_BTN_SHAPE_CREATE = "shape-create";
const ELEM_ID_BTN_SHAPE_DELETE = "shape-delete";
const ELEM_ID_BTN_SHAPE_DELETE_VERTEX = "shape-delete-vertex";
const ELEM_ID_BTN_SHAPE_BURNED = "shape-kind-burned";
const ELEM_ID_BTN_SHAPE_UNBURNED = "shape-kind-unburned";
const ELEM_ID_BTN_SHAPES_UPDATE = "shapes-update";

const SHAPE_KIND_UNBURNED = "unburned";
const SHAPE_KIND_BURNED = "burned";

/**
	* A location log
	* @typedef {Object} LocationMarker
	* @property {number} timestamp
	* @property {number} latitude
	* @property {number} longitude
	* @property {number} accuracy	 	- Accuracy in meters
	* @property {number} heading		- Compass heading in degress [0, 360]
*/

/**
	* A shape point
	* @typedef {Object} ShapePoint
	* @property {number} latitude
	* @property {number} longitude
*/

/**
	* A shape descriptor
	* @typedef {Object} ShapeDescriptor
	* @property {string} kind
	* @property {[]ShapePoint} points
*/

/**
	* A picture descriptor
	* @typedef {Object} PictureDescriptor
	* @property {string} filename
	* @property {number} latitude
	* @property {number} longitude
*/

/**
	* A shape
	* @typedef {Object} Shape
	* @property {string} kind
	* @property {[]ShapePoint} points
	* @property {[]Object} layers				- leaflet layers
	* @property {number} point_insert_idx		- index to start inserting points
*/

function lib_setup_handler_onclick(elementId, handler) {
	document.getElementById(elementId).onclick = handler
}

function lib_setup_map() {
	var map = L.map(ELEM_ID_MAP).setView(DEFAULT_COORDINATES, DEFAULT_ZOOM);
	L.Icon.Default.imagePath = "/static/";
	L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
		maxNativeZoom: 19,
		maxZoom: 25,
		attribution: '&copy; <a href="http://www.openstreetmap.org/copyright">OpenStreetMap</a>'
	}).addTo(map);
	return map;
}

/**
	* Fetch location logs
	* @return {Promise<LocationMarker[]>}
*/
async function lib_fetch_location_logs() {
	// const burned = [
	// 	[38.592177702929426, -9.145557060034113],
	// 	[38.58385651421202, -9.134116290522673],
	// 	[38.587516574932266, -9.134999747627804],
	// 	[38.59442184182009, -9.13809184749576],
	// 	[38.596734957715675, -9.138621921758839],
	// ];
	//
	// const unburned = [
	// 	[38.598388277527036, -9.135874396116632],
	// 	[38.589731317901276, -9.149692038446165],
	// 	[38.58043902375093, -9.138619879692945],
	// 	[38.591568658478, -9.12070962376425],
	// ];
	//
	// const location_logs = []
	// for (const point of burned.concat(unburned)) {
	// 	console.log(point)
	// 	location_logs.push({
	// 		latitude: point[0],
	// 		longitude: point[1],
	// 		accuracy: 5.8,
	// 		timestamp: 0,
	// 		heading: 0,
	// 	})
	// }
	// return Promise.resolve(location_logs);

	const response = await fetch("/api/location");
	return response.json();
}

/**
	* Fetch shape descriptors
	* @return {Promise<ShapeDescriptor[]>}
*/
async function lib_fetch_shape_descriptors() {
	const response = await fetch("/api/shapes");
	return response.json();
}

/**
	* Fetch picture descriptors
	* @return {Promise<PictureDescriptor[]>}
*/
async function lib_fetch_picture_descriptors() {
	const response = await fetch("/api/pictures");
	return response.json();
}

function lib_picture_descriptor_url(picture_descriptor) {
	const picture_url = `/picture/${picture_descriptor.filename}`
	return picture_url;
}

function lib_add_location_logs_to_map(map, locations) {
	for (const location of locations) {
		const len = 0.0002;
		const lat = Math.sin((location.heading + 90) * 2 * Math.PI / 360) * len;
		const lon = Math.cos((location.heading - 90) * 2 * Math.PI / 360) * len;
		const line_coords = [
			[location.latitude, location.longitude],
			[location.latitude + lat, location.longitude + lon],
		];
		L.circle([location.latitude, location.longitude], { radius: location.accuracy })
			.bindPopup(`${location.heading}`)
			.addTo(map);
		L.polyline(line_coords, { color: 'blue' }).addTo(map)
	}
}

function lib_shape_color_for_kind(kind) {
	if (kind == SHAPE_KIND_BURNED)
		return 'red';
	if (kind == SHAPE_KIND_UNBURNED)
		return 'green';
	return 'orange';
}

function lib_shape_create_empty() {
	return {
		kind: SHAPE_KIND_UNBURNED,
		points: [],
		layers: [],
		point_insert_idx: 0,
	};
}

function lib_shape_create_from_descriptor(desc) {
	return {
		kind: desc.kind,
		points: desc.points,
		layers: [],
		point_insert_idx: desc.points.length,
	};
}

function page_editor__on_shape_create(state) {
	const shape = lib_shape_create_empty();
	state.shapes.push(shape);
	page_editor__ui_shape_select(state, shape);
}

function page_editor__on_shape_delete(state) {
	if (state.selected_shape == null)
		return;
	page_editor__ui_shape_remove(state, state.selected_shape);
	state.shapes.splice(state.shapes.indexOf(state.selected_shape), 1);
	state.selected_shape = null;
}

function page_editor__on_shape_delete_vertex(state) {
	if (state.delete_selected_vertex_fn == null)
		return;
	state.delete_selected_vertex_fn();
}

function page_editor__on_shape_kind_unburned(state) {
	if (state.selected_shape == null)
		return;
	state.selected_shape.kind = SHAPE_KIND_UNBURNED;
	page_editor__ui_shape_add(state, state.selected_shape);
}

function page_editor__on_shape_kind_burned(state) {
	if (state.selected_shape == null)
		return;
	state.selected_shape.kind = SHAPE_KIND_BURNED;
	page_editor__ui_shape_add(state, state.selected_shape);
}

function page_editor__on_shapes_update(state) {
	const shape_descriptors = [];
	for (const shape of state.shapes) {
		if (shape.points.length < 3)
			continue;

		const points = [];
		for (const point of shape.points) {
			points.push({
				latitude: point.latitude,
				longitude: point.longitude,
			});
		}

		shape_descriptors.push({
			kind: shape.kind,
			points: points,
		});
	}

	fetch("/api/shapes", {
		method: "POST",
		body: JSON.stringify(shape_descriptors),
	}).then(() => {
		alert("updated");
		window.location.reload();
	}).catch((e) => {
		alert(`failed to update: ${e}`);
		window.location.reload();
	});
}

function page_editor__on_map_click(state, ev) {
	console.log("clicked on map");
	if (state.selected_shape == null)
		return;
	state.selected_shape.points.splice(state.selected_shape.point_insert_idx, 0, {
		latitude: ev.latlng.lat,
		longitude: ev.latlng.lng,
	});
	state.selected_shape.point_insert_idx += 1;
	page_editor__ui_shape_add(state, state.selected_shape);
}

function page_editor__setup_handlers(state) {
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_CREATE, () => page_editor__on_shape_create(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_DELETE, () => page_editor__on_shape_delete(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_DELETE_VERTEX, () => page_editor__on_shape_delete_vertex(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_UNBURNED, () => page_editor__on_shape_kind_unburned(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_BURNED, () => page_editor__on_shape_kind_burned(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPES_UPDATE, () => page_editor__on_shapes_update(state));

	state.map.on('click', (ev) => page_editor__on_map_click(state, ev));
}

function page_editor__ui_shape_remove(state, shape) {
	if (shape == null)
		return;
	for (const layer of shape.layers) {
		state.map.removeLayer(layer);
		layer.remove();
	}
	shape.layers = [];
}

function page_editor__ui_shape_select(state, shape) {
	const prev_shape = state.selected_shape;
	state.selected_shape = shape;
	page_editor__ui_shape_add(state, shape);
	if (prev_shape != null)
		page_editor__ui_shape_add(state, prev_shape);
}

function page_editor__ui_shape_add(state, shape) {
	if (shape == null)
		return;

	page_editor__ui_shape_remove(state, shape);

	const selected = state.selected_shape == shape;
	const color = lib_shape_color_for_kind(shape.kind);
	const positions = [];
	for (var i = 0; i < shape.points.length; i += 1) {
		const point = shape.points[i];
		const highlight_point = (shape.point_insert_idx - 1 == i) && selected;
		const coords = [point.latitude, point.longitude];
		const circle_color = highlight_point ? 'blue' : 'red';
		const circle_idx = i;
		console.assert(point.latitude != null, "invalid point latitude")
		console.assert(point.longitude != null, "invalid point longitude")

		const remove_circle = () => {
			shape.points.splice(circle_idx, 1);
			shape.point_insert_idx = circle_idx;
			page_editor__ui_shape_add(state, shape);
		};
		const update_insert_idx = () => {
			shape.point_insert_idx = circle_idx + 1;
			page_editor__ui_shape_add(state, shape);
		};

		if (highlight_point)
			state.delete_selected_vertex_fn = remove_circle;

		positions.push(coords);
		const circle = L.circle(coords, { radius: state.vertex_radius, color: circle_color, bubblingMouseEvents: false })
			.on('click', (e) => {
				if (e.originalEvent.shiftKey) {
					remove_circle();
				} else {
					update_insert_idx();
				}
			})
			.on('contextmenu', () => remove_circle())
			.addTo(state.map);
		shape.layers.push(circle);

		if (selected) {
			const tooltip = L.tooltip(coords, { content: `${i}` })
				.addTo(state.map);
			shape.layers.push(tooltip);
		}
	}

	if (positions.length >= 3) {
		const opacity = state.selected_shape == shape ? 0.2 : 0.04;
		const select = () => page_editor__ui_shape_select(state, shape);
		const poly = L.polygon(positions, { color: color, fillOpacity: opacity })
			.on('click', (ev) => { if (ev.originalEvent.shiftKey) { L.DomEvent.stopPropagation(ev); select(); } })
			.on('dblclick', (ev) => {
				L.DomEvent.stopPropagation(ev);
				select();
			})
			.addTo(state.map);
		shape.layers.push(poly);
	}
}

async function page_editor__main() {
	const map = lib_setup_map();
	const locations = await lib_fetch_location_logs();
	const shape_descriptors = await lib_fetch_shape_descriptors();

	const state = {
		map: map,
		locations: locations,
		shapes: [],
		selected_shape: null,
		delete_selected_vertex_fn: null,
		vertex_radius: 15,
	};
	window.state = state; // to allow access from the console

	page_editor__setup_handlers(state);
	lib_add_location_logs_to_map(state.map, state.locations);

	const vertex_radius_slider = document.getElementById("shape-vertex-radius");
	vertex_radius_slider.addEventListener("change", () => {
		state.vertex_radius = vertex_radius_slider.value;
		page_editor__ui_shape_add(state, state.selected_shape);
	});

	for (const descriptor of shape_descriptors) {
		const shape = lib_shape_create_from_descriptor(descriptor);
		state.shapes.push(shape);
		page_editor__ui_shape_add(state, shape);
	}
}

function page_index__poly_create_from_shape_descriptor(map, shape_descriptor) {
	const color = lib_shape_color_for_kind(shape_descriptor.kind);
	const points = []
	for (const point of shape_descriptor.points) {
		points.push([point.latitude, point.longitude]);
	}
	L.polygon(points, { color: color }).addTo(map);
}

function page_index__create_image_popup(picture_descriptor) {
	const e = document.getElementById("image-frame");
	if (e != null)
		e.remove();

	const d = document.createElement("div");
	d.id = "image-frame";
	const i = document.createElement("img");
	i.src = lib_picture_descriptor_url(picture_descriptor);

	d.onclick = () => {
		d.remove();
	};

	d.appendChild(i);
	document.body.appendChild(d);
}

function page_index__add_picture_descriptor_to_map(map, picture_descriptor) {
	L.marker([picture_descriptor.latitude, picture_descriptor.longitude])
		.on('click', () => {
			page_index__create_image_popup(picture_descriptor);
		})
		.addTo(map)
}

async function page_index__main() {
	const map = lib_setup_map();
	const [shape_descriptors, picture_descriptors] = await Promise.all([
		lib_fetch_shape_descriptors(),
		lib_fetch_picture_descriptors(),
	]);
	for (const descriptor of shape_descriptors) {
		page_index__poly_create_from_shape_descriptor(map, descriptor);
	}
	for (const descriptor of picture_descriptors) {
		page_index__add_picture_descriptor_to_map(map, descriptor);
	}

	setTimeout(() => {
		console.log("create div");
	}, 1000);
}
