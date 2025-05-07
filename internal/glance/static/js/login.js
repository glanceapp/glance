import { find } from "./templating.js";

const AUTH_ENDPOINT = pageData.baseURL + "/api/authenticate";

const showPasswordSVG = `<svg class="form-input-icon" stroke="var(--color-text-base)" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5">
    <path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
</svg>`;

const hidePasswordSVG = `<svg class="form-input-icon" stroke="var(--color-text-base)" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5">
    <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
    <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
</svg>`;

const container = find("#login-container");
const usernameInput = find("#username");
const passwordInput = find("#password");
const errorMessage = find("#error-message");
const loginButton = find("#login-button");
const toggleVisibilityButton = find("#toggle-password-visibility");

const state = {
    lastUsername: "",
    lastPassword: "",
    isLoading: false,
    isRateLimited: false
};

const lang = {
    showPassword: "Show password",
    hidePassword: "Hide password",
    incorrectCredentials: "Incorrect username or password",
    rateLimited: "Too many login attempts, try again in a few minutes",
    unknownError: "An error occurred, please try again",
};

container.clearStyles("display");
setTimeout(() => usernameInput.focus(), 200);

toggleVisibilityButton
    .html(showPasswordSVG)
    .attr("title", lang.showPassword)
    .on("click", function() {
        if (passwordInput.type === "password") {
            passwordInput.type = "text";
            toggleVisibilityButton.html(hidePasswordSVG).attr("title", lang.hidePassword);
            return;
        }

        passwordInput.type = "password";
        toggleVisibilityButton.html(showPasswordSVG).attr("title", lang.showPassword);
    });

function enableLoginButtonIfCriteriaMet() {
    const usernameValue = usernameInput.value.trim();
    const passwordValue = passwordInput.value.trim();

    const usernameValid = usernameValue.length >= 3;
    const passwordValid = passwordValue.length >= 6;

    const isUsingLastCredentials =
           usernameValue === state.lastUsername
        && passwordValue === state.lastPassword;

    loginButton.disabled = !(
           usernameValid
        && passwordValid
        && !isUsingLastCredentials
        && !state.isLoading
        && !state.isRateLimited
    );
}

usernameInput.on("input", enableLoginButtonIfCriteriaMet);
passwordInput.on("input", enableLoginButtonIfCriteriaMet);

async function handleLoginAttempt() {
    state.lastUsername = usernameInput.value;
    state.lastPassword = passwordInput.value;
    errorMessage.text("");

    loginButton.disable();
    state.isLoading = true;

    const response = await fetch(AUTH_ENDPOINT, {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            username: usernameInput.value,
            password: passwordInput.value
        }),
    });

    state.isLoading = false;
    if (response.status === 200) {
        setTimeout(() => { window.location.href = pageData.baseURL + "/"; }, 300);

        container.animate({
            keyframes: [{ offset: 1, transform: "scale(0.95)", opacity: 0 }],
            options: { duration: 300, easing: "ease", fill: "forwards" }}
        );

        find("footer")?.animate({
            keyframes: [{ offset: 1, opacity: 0 }],
            options: { duration: 300, easing: "ease", fill: "forwards", delay: 50 }
        });
    } else if (response.status === 401) {
        errorMessage.text(lang.incorrectCredentials);
        passwordInput.focus();
    } else if (response.status === 429) {
        errorMessage.text(lang.rateLimited);
        state.isRateLimited = true;
        const retryAfter = response.headers.get("Retry-After") || 30;
        setTimeout(() => {
            state.lastUsername = "";
            state.lastPassword = "";
            state.isRateLimited = false;

            enableLoginButtonIfCriteriaMet();
        }, retryAfter * 1000);
    } else {
        errorMessage.text(lang.unknownError);
        passwordInput.focus();
    }
}

loginButton.disable().on("click", handleLoginAttempt);
