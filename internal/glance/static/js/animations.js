export const easeOutQuint = 'cubic-bezier(0.22, 1, 0.36, 1)';

export function directions(anim, opt, ...dirs) {
    return dirs.map(dir => anim({ direction: dir, ...opt }));
}

export function slideFade({
    direction = 'left',
    fill = 'backwards',
    duration = 200,
    distance = '1rem',
    easing = 'ease',
    offset = 0,
}) {
    const axis = direction === 'left' || direction === 'right' ? 'X' : 'Y';
    const negative = direction === 'left' || direction === 'up' ? '-' : '';
    const amount = negative + distance;

    return {
        keyframes: [
            {
                offset: offset,
                opacity: 0,
                transform: `translate${axis}(${amount})`,
            }
        ],
        options: {
            duration: duration,
            easing: easing,
            fill: fill,
        },
    };
}
