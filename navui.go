package greener

var NavUISupport []UISupport

func init() {
	NavUISupport = append(NavUISupport, NewDefaultUISupport(
		`
/* Breadcrumb Navigation */
ul.breadcrumbs {
    list-style: none;
    padding: 0;
    margin: 0;
    white-space: nowrap; /* Prevents wrapping */
}

ul.breadcrumbs li {
    display: inline-block; /* Places list items in a line */
    margin-right: 5px; /* Spacing to the right of each item */
    position: relative;
}

ul.breadcrumbs li:not(:last-child)::after {
    content: ">"; /* Separator */
    margin-left: 12px;
    color: #666;
}

ul.breadcrumbs li.active {
    font-weight: bold;
    color: #333;
}


/* Section navigation */
ul.section, ul.section ul {
    list-style: none; /* Removes default list styling */
    padding: 0;
    margin: 0;
}

ul.section li, ul.section ul li {
    background-color: #f8f8f8; /* Light gray background for all items */
    padding: 8px 15px; /* Padding for some spacing around text */
    border-bottom: 1px solid #e7e7e7; /* Separator between items */
}

ul.section li:hover, ul.section ul li:hover {
    background-color: #e7e7e7; /* Slightly darker background on hover */
}

ul.section ul {
    padding-left: 20px; /* Indents nested ul */
}

ul.section ul li {
    background-color: #e9eff5; /* Different background for nested items */
}
ul.section li.section::after {
    content: " >"; /* Adds a greater-than sign as a separator */
}

`,
		`console.log("Hello from script");`,
		`console.log("Hello from service worker");`,
	))
}
