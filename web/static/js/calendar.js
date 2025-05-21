import { directions, easeOutQuint, slideFade } from "./animations.js";
import { elem, repeat, text } from "./templating.js";

const FULL_MONTH_SLOTS = 7*6;
const WEEKDAY_ABBRS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];
const MONTH_NAMES = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

const leftArrowSvg = `<svg stroke="var(--color-text-base)" fill="none" viewBox="0 0 24 24" stroke-width="1.5" xmlns="http://www.w3.org/2000/svg">
  <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
</svg>`;

const rightArrowSvg = `<svg stroke="var(--color-text-base)" fill="none" viewBox="0 0 24 24" stroke-width="1.5" xmlns="http://www.w3.org/2000/svg">
  <path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
</svg>`;

const undoArrowSvg = `<svg stroke="var(--color-text-base)" fill="none" viewBox="0 0 24 24" stroke-width="1.5" xmlns="http://www.w3.org/2000/svg">
  <path stroke-linecap="round" stroke-linejoin="round" d="M9 15 3 9m0 0 6-6M3 9h12a6 6 0 0 1 0 12h-3" />
</svg>`;

const [datesExitLeft, datesExitRight] = directions(
    slideFade, { distance: "2rem", duration: 120, offset: 1 },
    "left", "right"
);

const [datesEntranceLeft, datesEntranceRight] = directions(
    slideFade, { distance: "0.8rem", duration: 500, easing: easeOutQuint },
    "left", "right"
);

const undoEntrance = slideFade({ direction: "left", distance: "100%", duration: 300 });

export default function(element) {
    element.swapWith(Calendar(
        Number(element.dataset.firstDayOfWeek ?? 1)
    ));
}

// TODO: when viewing the previous/next month, display the current date if it's within the spill-over days
function Calendar(firstDay) {
    let header, dates;
    let advanceTimeTicker;
    let now = new Date();
    let activeDate;

    const update = (newDate) => {
        header.component.update(now, newDate);
        dates.component.update(now, newDate);
        activeDate = newDate;
    };

    const autoAdvanceNow = () => {
        advanceTimeTicker = setTimeout(() => {
            // TODO: don't auto advance if looking at a different month
            update(now = new Date());
            autoAdvanceNow();
        }, msTillNextDay());
    };

    const adjacentMonth = (dir) => new Date(activeDate.getFullYear(), activeDate.getMonth() + dir, 1);
    const nextClicked = () => update(adjacentMonth(1));
    const prevClicked = () => update(adjacentMonth(-1));
    const undoClicked = () => update(now);

    const calendar = elem().classes("calendar").append(
        header = Header(nextClicked, prevClicked, undoClicked),
        dates = Dates(firstDay)
    );

    update(now);
    autoAdvanceNow();

    return calendar.component({
        suspend: () => clearTimeout(advanceTimeTicker)
    });
}

function Header(nextClicked, prevClicked, undoClicked) {
    let month, monthNumber, year, undo;
    const button = () => elem("button").classes("calendar-header-button");

    const monthAndYear = elem().classes("size-h2", "color-highlight").append(
        month = text(),
        " ",
        year = elem("span").classes("size-h3"),
        undo = button()
            .hide()
            .classes("calendar-undo-button")
            .attr("title", "Back to current month")
            .on("click", undoClicked)
            .html(undoArrowSvg)
    );

    const monthSwitcher = elem()
        .classes("flex", "gap-7", "items-center")
        .append(
            button()
                .attr("title", "Previous month")
                .on("click", prevClicked)
                .html(leftArrowSvg),
            monthNumber = elem()
                .classes("color-highlight")
                .styles({ marginTop: "0.1rem" }),
            button()
                .attr("title", "Next month")
                .on("click", nextClicked)
                .html(rightArrowSvg),
        );

    return elem().classes("flex", "justify-between", "items-center").append(
        monthAndYear,
        monthSwitcher
    ).component({
        update: function (now, newDate) {
            month.text(MONTH_NAMES[newDate.getMonth()]);
            year.text(newDate.getFullYear());
            const m = newDate.getMonth() + 1;
            monthNumber.text((m < 10 ? "0" : "") + m);

            if (!datesWithinSameMonth(now, newDate)) {
                if (undo.isHidden()) undo.show().animate(undoEntrance);
            } else {
                undo.hide();
            }

            return this;
        }
    });
}

function Dates(firstDay) {
    let dates, lastRenderedDate;

    const updateFullMonth = function(now, newDate) {
        const firstWeekday = new Date(newDate.getFullYear(), newDate.getMonth(), 1).getDay();
        const previousMonthSpilloverDays = (firstWeekday - firstDay + 7) % 7 || 7;
        const currentMonthDays = daysInMonth(newDate.getFullYear(), newDate.getMonth());
        const nextMonthSpilloverDays = FULL_MONTH_SLOTS - (previousMonthSpilloverDays + currentMonthDays);
        const previousMonthDays = daysInMonth(newDate.getFullYear(), newDate.getMonth() - 1)
        const isCurrentMonth = datesWithinSameMonth(now, newDate);
        const currentDate = now.getDate();

        let children = dates.children;
        let index = 0;

        for (let i = 0; i < FULL_MONTH_SLOTS; i++) {
            children[i].clearClasses("calendar-spillover-date", "calendar-current-date");
        }

        for (let i = 0; i < previousMonthSpilloverDays; i++, index++) {
            children[index].classes("calendar-spillover-date").text(
                previousMonthDays - previousMonthSpilloverDays + i + 1
            )
        }

        for (let i = 1; i <= currentMonthDays; i++, index++) {
            children[index]
                .classesIf(isCurrentMonth && i === currentDate, "calendar-current-date")
                .text(i);
        }

        for (let i = 0; i < nextMonthSpilloverDays; i++, index++) {
            children[index].classes("calendar-spillover-date").text(i + 1);
        }

        lastRenderedDate = newDate;
    };

    const update = function(now, newDate) {
        if (lastRenderedDate === undefined || datesWithinSameMonth(newDate, lastRenderedDate)) {
            updateFullMonth(now, newDate);
            return;
        }

        const next = newDate > lastRenderedDate;
        dates.animateUpdate(
            () => updateFullMonth(now, newDate),
            next ? datesExitLeft : datesExitRight,
            next ? datesEntranceRight : datesEntranceLeft,
        );
    }

    return elem().append(
        elem().classes("calendar-dates", "margin-top-15").append(
            ...repeat(7, (i) => elem().classes("size-h6", "color-subdue").text(
                WEEKDAY_ABBRS[(firstDay + i) % 7]
            ))
        ),

        dates = elem().classes("calendar-dates", "margin-top-3").append(
            ...elem().classes("calendar-date").duplicate(FULL_MONTH_SLOTS)
        )
    ).component({ update });
}

function datesWithinSameMonth(d1, d2) {
    return d1.getFullYear() === d2.getFullYear() && d1.getMonth() === d2.getMonth();
}

function daysInMonth(year, month) {
    return new Date(year, month + 1, 0).getDate();
}

function msTillNextDay(now) {
    now = now || new Date();

    return 86_400_000 - (
      now.getMilliseconds() +
      now.getSeconds() * 1000 +
      now.getMinutes() * 60_000 +
      now.getHours() * 3_600_000
    );
}
