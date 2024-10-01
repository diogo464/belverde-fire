const DEFAULT_ZOOM = 14;
const DEFAULT_COORDINATES = [38.59104623572979, -9.130882470026634];

const ELEM_ID_MAP = "map";
const ELEM_ID_BTN_SHAPE_CREATE = "shape-create";
const ELEM_ID_BTN_SHAPE_DELETE = "shape-delete";
const ELEM_ID_BTN_SHAPE_BURNED = "shape-kind-burned";
const ELEM_ID_BTN_SHAPE_UNBURNED = "shape-kind-unburned";
const ELEM_ID_BTN_SHAPES_UPDATE = "shapes-update";

const SHAPE_KIND_UNBURNED = "unburned";
const SHAPE_KIND_BURNED = "burned";

/**
	* A location log
	* @typedef {Object} LocationLog
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
	* A shape
	* @typedef {Object} Shape
	* @property {string} kind
	* @property {[]ShapePoint} points
	* @property {Object} poly					- leaflet polygon, null if points.length < 3
	* @property {[]Object} poly_points			- leaflet circles for each point
	* @property {number} point_insert_idx		- index to start inserting points
*/

function lib_setup_handler_onclick(elementId, handler) {
	document.getElementById(elementId).onclick = handler
}

function lib_setup_map() {
	var map = L.map(ELEM_ID_MAP).setView(DEFAULT_COORDINATES, DEFAULT_ZOOM);
	L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
		maxZoom: 19,
		attribution: '&copy; <a href="http://www.openstreetmap.org/copyright">OpenStreetMap</a>'
	}).addTo(map);
	return map;
}

/**
	* Fetch location logs
	* @return {Promise<LocationLog[]>}
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
	* Fetch location logs
	* @return {Promise<ShapeDescriptor[]>}
*/
async function lib_fetch_shape_descriptors() {
	const response = await fetch("/api/shapes");
	return response.json();
}

function lib_add_location_logs_to_map(map, locations) {
	for (const location of locations) {
		const len = 0.0002;
		const lat = Math.sin((location.heading + 90) * 2 * Math.PI / 360) * len;
		const lon = Math.cos((location.heading + 90) * 2 * Math.PI / 360) * len;
		const line_coords = [
			[location.latitude, location.longitude],
			[location.latitude + lat, location.longitude + lon],
		];
		L.circle([location.latitude, location.longitude], { radius: location.accuracy }).addTo(map);
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
		poly: null,
		poly_points: [],
		point_insert_idx: 0,
	};
}

function lib_shape_create_from_descriptor(desc) {
	return {
		kind: desc.kind,
		points: desc.points,
		poly: null,
		poly_points: [],
		point_insert_idx: desc.points.length,
	};
}

function page_shape__on_shape_create(state) {
	const shape = lib_shape_create_empty();
	state.shapes.push(shape);
	page_shape__ui_shape_select(state, shape);
}

function page_shape__on_shape_delete(state) {
	if (state.selected_shape == null)
		return;
	page_shape__ui_shape_remove(state, state.selected_shape);
	state.shapes.splice(state.shapes.indexOf(state.selected_shape), 1);
	state.selected_shape = null;
}

function page_shape__on_shape_kind_unburned(state) {
	if (state.selected_shape == null)
		return;
	state.selected_shape.kind = SHAPE_KIND_UNBURNED;
	page_shape__ui_shape_add(state, state.selected_shape);
}

function page_shape__on_shape_kind_burned(state) {
	if (state.selected_shape == null)
		return;
	state.selected_shape.kind = SHAPE_KIND_BURNED;
	page_shape__ui_shape_add(state, state.selected_shape);
}

function page_shape__on_shapes_update(state) {
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

function page_shape__on_map_click(state, ev) {
	console.log("clicked on map");
	if (state.selected_shape == null)
		return;
	state.selected_shape.points.splice(state.selected_shape.point_insert_idx, 0, {
		latitude: ev.latlng.lat,
		longitude: ev.latlng.lng,
	});
	state.selected_shape.point_insert_idx += 1;
	page_shape__ui_shape_add(state, state.selected_shape);
}

function page_shape__setup_handlers(state) {
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_CREATE, () => page_shape__on_shape_create(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_DELETE, () => page_shape__on_shape_delete(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_UNBURNED, () => page_shape__on_shape_kind_unburned(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPE_BURNED, () => page_shape__on_shape_kind_burned(state));
	lib_setup_handler_onclick(ELEM_ID_BTN_SHAPES_UPDATE, () => page_shape__on_shapes_update(state));

	state.map.on('click', (ev) => page_shape__on_map_click(state, ev));
}

function page_shape__ui_shape_remove(state, shape) {
	for (const circle of shape.poly_points) {
		state.map.removeLayer(circle);
	}
	shape.poly_points = [];

	if (shape.poly != null) {
		state.map.removeLayer(shape.poly);
		shape.poly.remove();
		shape.poly = null;
	}
}

function page_shape__ui_shape_select(state, shape) {
	const prev_shape = state.selected_shape;
	state.selected_shape = shape;
	page_shape__ui_shape_add(state, shape);
	if (prev_shape != null)
		page_shape__ui_shape_add(state, prev_shape);
}

function page_shape__ui_shape_add(state, shape) {
	page_shape__ui_shape_remove(state, shape);

	const color = lib_shape_color_for_kind(shape.kind);
	const positions = [];
	for (var i = 0; i < shape.points.length; i += 1) {
		const point = shape.points[i];
		const highlight_point = shape.point_insert_idx - 1 == i;
		const coords = [point.latitude, point.longitude];
		const circle_color = highlight_point ? 'blue' : 'red';
		const circle_idx = i;
		console.assert(point.latitude != null, "invalid point latitude")
		console.assert(point.longitude != null, "invalid point longitude")

		positions.push(coords);
		const circle = L.circle(coords, { radius: 15, color: circle_color, bubblingMouseEvents: false })
			.on('click', (e) => {
				if (e.originalEvent.shiftKey) {
					shape.points.splice(circle_idx, 1);
					shape.point_insert_idx = circle_idx;
					page_shape__ui_shape_add(state, shape);
				} else {
					console.log(`clicked on circle, setting point insert idx to ${circle_idx + 1}`);
					shape.point_insert_idx = circle_idx + 1;
					page_shape__ui_shape_add(state, shape);
				}
			})
			.addTo(state.map);
		shape.poly_points.push(circle);
	}

	if (positions.length >= 3) {
		const opacity = state.selected_shape == shape ? 0.2 : 0.04;
		shape.poly = L.polygon(positions, { color: color, fillOpacity: opacity, bubblingMouseEvents: false })
			.on('click', () => {
				page_shape__ui_shape_select(state, shape);
			})
			.addTo(state.map);
	}
}

async function page_shape__main() {
	const map = lib_setup_map();
	const locations = await lib_fetch_location_logs();
	const shape_descriptors = await lib_fetch_shape_descriptors();

	const state = {
		map: map,
		locations: locations,
		shapes: [],
		selected_shape: null,
	};
	window.state = state; // to allow access from the console

	page_shape__setup_handlers(state);
	lib_add_location_logs_to_map(state.map, state.locations);

	for (const descriptor of shape_descriptors) {
		const shape = lib_shape_create_from_descriptor(descriptor);
		state.shapes.push(shape);
		page_shape__ui_shape_add(state, shape);
	}
}

function page_main__poly_create_from_shape_descriptor(map, shape_descriptor) {
	const color = lib_shape_color_for_kind(shape_descriptor.kind);
	const points = []
	for (const point of shape_descriptor.points) {
		points.push([point.latitude, point.longitude]);
	}
	L.polygon(points, { color: color }).addTo(map);
}

async function page_index__main() {
	const map = lib_setup_map();
	const shape_descriptors = await lib_fetch_shape_descriptors();
	for (const descriptor of shape_descriptors) {
		page_main__poly_create_from_shape_descriptor(map, descriptor);
	}
}
