import math
import numpy as np
import matplotlib.pyplot as plt
from scipy.optimize import minimize
import json
import sys

# Read data from speedrun_data.json
with open('speedrun_data.json', 'r') as file:
    data = json.load(file)

# Extract daysAfterRelease (x) and time (y)
# Read index from command line arguments
if len(sys.argv) > 1:
    activity_id = int(sys.argv[1])
else:
    raise ValueError("Please provide an activity_id to the data in speedrun_data.json")

runs = [item for item in data if item["activity_id"] == activity_id][0]["runs"]
x_data = np.array([run["days_after_release"] for run in runs])
y_data = np.array([run["time"] for run in runs])


def curve(x, a, b, c, d):
    return a - b * np.log10((x + c) ** d)

# Objective function for optimization
def objective(params):
    a, b, c, d = params
    y_curve = curve(x_data, a, b, c, d)

    # Penalize when the curve goes above the points
    above_penalty = np.sum((y_curve - y_data)[y_curve > y_data] ** 4)
    # Penalize when the curve goes too far below the points\
    below_penalty = np.sum((y_data - y_curve)[y_curve < y_data] ** 2)


    return above_penalty + below_penalty

# Initial guesses for the parameters
initial_guess = [600, 50, 1, 1]  # a, b, c, d

# Perform optimization
result = minimize(objective, initial_guess, bounds=[(200, 3000), (1, 150), (1, 5), (0.5, 10), ])
a_opt, b_opt, c_opt, d_opt = result.x

# Generate curve points
x_fit = np.linspace(min(x_data), max(x_data), 500)
y_fit = curve(x_fit, a_opt, b_opt, c_opt, d_opt)

# Plot the results
plt.scatter(x_data, y_data, label="Data Points", color="red")
plt.plot(x_fit, y_fit, label=f"Fitted Curve: a={a_opt:.2f}, b={b_opt:.2f}, c={c_opt:.2f}, d={d_opt:.2f}", color="blue")
plt.ylim(bottom=0)  # Set the y-axis to start from 0
# make top of y axis a bit higher than the highest point
plt.ylim(top=max(y_data) + 100)
plt.legend()
plt.xlabel("Days After Release")
plt.ylabel("Time")
plt.title("Curve Fitting for Speedrun Data")
plt.grid()
plt.savefig('../plot.png')

# print equation in latex format
print(f"y = {a_opt:.2f} - {b_opt:.2f} \log_{{10}}((x + {c_opt:.2f})^{d_opt:.2f})")
# print out the golang function, like
# SpeedrunCurve: func (daysAfterRelease float64) float64 {
#     return 600 - 50 * math.Log(math.Pow(daysAfterRelease + 1, 1))
# },
print(f"SpeedrunCurve: func (daysAfterRelease float64) float64 {{\n\treturn {a_opt:.2f} - ({b_opt:.2f} * math.Log10(math.Pow(daysAfterRelease + {c_opt:.2f}, {d_opt:.2f})))\n}},")

